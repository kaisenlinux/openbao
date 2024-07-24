// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	"github.com/openbao/openbao/helper/forwarding"
	"github.com/openbao/openbao/sdk/helper/consts"
	"github.com/openbao/openbao/vault/cluster"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type requestForwardingHandler struct {
	fws         *http2.Server
	fwRPCServer *grpc.Server
	logger      log.Logger
	ha          bool
	core        *Core
	stopCh      chan struct{}
}

type requestForwardingClusterClient struct {
	core *Core
}

// NewRequestForwardingHandler creates a cluster handler for use with request
// forwarding.
func NewRequestForwardingHandler(c *Core, fws *http2.Server) (*requestForwardingHandler, error) {
	// Resolve locally to avoid races
	ha := c.ha != nil

	fwRPCServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time: 2 * c.clusterHeartbeatInterval,
		}),
		grpc.MaxRecvMsgSize(math.MaxInt32),
		grpc.MaxSendMsgSize(math.MaxInt32),
	)

	if ha && c.clusterHandler != nil {
		RegisterRequestForwardingServer(fwRPCServer, &forwardedRequestRPCServer{
			core:               c,
			handler:            c.clusterHandler,
			raftFollowerStates: c.raftFollowerStates,
		})
	}

	return &requestForwardingHandler{
		fws:         fws,
		fwRPCServer: fwRPCServer,
		ha:          ha,
		logger:      c.logger.Named("request-forward"),
		core:        c,
		stopCh:      make(chan struct{}),
	}, nil
}

// ClientLookup satisfies the ClusterClient interface and returns the ha tls
// client certs.
func (c *requestForwardingClusterClient) ClientLookup(ctx context.Context, requestInfo *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	parsedCert := c.core.localClusterParsedCert.Load().(*x509.Certificate)
	if parsedCert == nil {
		return nil, nil
	}
	currCert := c.core.localClusterCert.Load().([]byte)
	if len(currCert) == 0 {
		return nil, nil
	}
	localCert := make([]byte, len(currCert))
	copy(localCert, currCert)

	for _, subj := range requestInfo.AcceptableCAs {
		if bytes.Equal(subj, parsedCert.RawIssuer) {
			return &tls.Certificate{
				Certificate: [][]byte{localCert},
				PrivateKey:  c.core.localClusterPrivateKey.Load().(*ecdsa.PrivateKey),
				Leaf:        c.core.localClusterParsedCert.Load().(*x509.Certificate),
			}, nil
		}
	}

	return nil, nil
}

func (c *requestForwardingClusterClient) ServerName() string {
	parsedCert := c.core.localClusterParsedCert.Load().(*x509.Certificate)
	if parsedCert == nil {
		return ""
	}

	return parsedCert.Subject.CommonName
}

func (c *requestForwardingClusterClient) CACert(ctx context.Context) *x509.Certificate {
	return c.core.localClusterParsedCert.Load().(*x509.Certificate)
}

// ServerLookup satisfies the ClusterHandler interface and returns the server's
// tls certs.
func (rf *requestForwardingHandler) ServerLookup(ctx context.Context, clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	currCert := rf.core.localClusterCert.Load().([]byte)
	if len(currCert) == 0 {
		return nil, fmt.Errorf("got forwarding connection but no local cert")
	}

	localCert := make([]byte, len(currCert))
	copy(localCert, currCert)

	return &tls.Certificate{
		Certificate: [][]byte{localCert},
		PrivateKey:  rf.core.localClusterPrivateKey.Load().(*ecdsa.PrivateKey),
		Leaf:        rf.core.localClusterParsedCert.Load().(*x509.Certificate),
	}, nil
}

// CALookup satisfies the ClusterHandler interface and returns the ha ca cert.
func (rf *requestForwardingHandler) CALookup(ctx context.Context) ([]*x509.Certificate, error) {
	parsedCert := rf.core.localClusterParsedCert.Load().(*x509.Certificate)

	if parsedCert == nil {
		return nil, fmt.Errorf("forwarding connection client but no local cert")
	}

	return []*x509.Certificate{parsedCert}, nil
}

// Handoff serves a request forwarding connection.
func (rf *requestForwardingHandler) Handoff(ctx context.Context, shutdownWg *sync.WaitGroup, closeCh chan struct{}, tlsConn *tls.Conn) error {
	if !rf.ha {
		tlsConn.Close()
		return nil
	}

	rf.logger.Debug("got request forwarding connection")

	shutdownWg.Add(2)
	// quitCh is used to close the connection and the second
	// goroutine if the server closes before closeCh.
	quitCh := make(chan struct{})
	go func() {
		select {
		case <-quitCh:
		case <-closeCh:
		case <-rf.stopCh:
		}
		tlsConn.Close()
		shutdownWg.Done()
	}()

	go func() {
		rf.fws.ServeConn(tlsConn, &http2.ServeConnOpts{
			Handler: rf.fwRPCServer,
			BaseConfig: &http.Server{
				ErrorLog: rf.logger.StandardLogger(nil),
			},
		})

		// close the quitCh which will close the connection and
		// the other goroutine.
		close(quitCh)
		shutdownWg.Done()
	}()

	return nil
}

// Stop stops the request forwarding server and closes connections.
func (rf *requestForwardingHandler) Stop() error {
	// Give some time for existing RPCs to drain.
	time.Sleep(cluster.ListenerAcceptDeadline)
	close(rf.stopCh)
	rf.fwRPCServer.Stop()
	return nil
}

// Starts the listeners and servers necessary to handle forwarded requests
func (c *Core) startForwarding(ctx context.Context) error {
	c.logger.Debug("request forwarding setup function")
	defer c.logger.Debug("leaving request forwarding setup function")

	// Clean up in case we have transitioned from a client to a server
	c.requestForwardingConnectionLock.Lock()
	c.clearForwardingClients()
	c.requestForwardingConnectionLock.Unlock()

	clusterListener := c.getClusterListener()
	if c.ha == nil || clusterListener == nil {
		c.logger.Debug("request forwarding not setup")
		return nil
	}

	handler, err := NewRequestForwardingHandler(c, clusterListener.Server())
	if err != nil {
		return err
	}

	clusterListener.AddHandler(consts.RequestForwardingALPN, handler)

	return nil
}

func (c *Core) stopForwarding() {
	clusterListener := c.getClusterListener()
	if clusterListener != nil {
		clusterListener.StopHandler(consts.RequestForwardingALPN)
	}
}

// refreshRequestForwardingConnection ensures that the client/transport are
// alive and that the current active address value matches the most
// recently-known address.
func (c *Core) refreshRequestForwardingConnection(ctx context.Context, clusterAddr string) error {
	c.logger.Debug("refreshing forwarding connection", "clusterAddr", clusterAddr)
	defer c.logger.Debug("done refreshing forwarding connection", "clusterAddr", clusterAddr)

	c.requestForwardingConnectionLock.Lock()
	defer c.requestForwardingConnectionLock.Unlock()

	// Clean things up first
	c.clearForwardingClients()

	// If we don't have anything to connect to, just return
	if clusterAddr == "" {
		return nil
	}

	clusterURL, err := url.Parse(clusterAddr)
	if err != nil {
		c.logger.Error("error parsing cluster address attempting to refresh forwarding connection", "error", err)
		return err
	}

	parsedCert := c.localClusterParsedCert.Load().(*x509.Certificate)
	if parsedCert == nil {
		c.logger.Error("no request forwarding cluster certificate found")
		return errors.New("no request forwarding cluster certificate found")
	}

	clusterListener := c.getClusterListener()
	if clusterListener == nil {
		c.logger.Error("no cluster listener configured")
		return nil
	}

	clusterListener.AddClient(consts.RequestForwardingALPN, &requestForwardingClusterClient{
		core: c,
	})

	// Set up grpc forwarding handling
	// It's not really insecure, but we have to dial manually to get the
	// ALPN header right. It's just "insecure" because GRPC isn't managing
	// the TLS state.
	dctx, cancelFunc := context.WithCancel(ctx)
	c.rpcClientConn, err = grpc.DialContext(dctx, clusterURL.Host,
		grpc.WithDialer(clusterListener.GetDialerFunc(ctx, consts.RequestForwardingALPN)),
		grpc.WithInsecure(), // it's not, we handle it in the dialer
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time: 2 * c.clusterHeartbeatInterval,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt32),
			grpc.MaxCallSendMsgSize(math.MaxInt32),
		))
	if err != nil {
		cancelFunc()
		c.logger.Error("err setting up forwarding rpc client", "error", err)
		return err
	}
	c.rpcClientConnContext = dctx
	c.rpcClientConnCancelFunc = cancelFunc
	c.rpcForwardingClient = &forwardingClient{
		RequestForwardingClient: NewRequestForwardingClient(c.rpcClientConn),
		core:                    c,
		echoTicker:              time.NewTicker(c.clusterHeartbeatInterval),
		echoContext:             dctx,
	}
	c.rpcForwardingClient.startHeartbeat()

	return nil
}

func (c *Core) clearForwardingClients() {
	c.logger.Debug("clearing forwarding clients")
	defer c.logger.Debug("done clearing forwarding clients")

	if c.rpcClientConnCancelFunc != nil {
		c.rpcClientConnCancelFunc()
		c.rpcClientConnCancelFunc = nil
	}
	if c.rpcClientConn != nil {
		c.rpcClientConn.Close()
		c.rpcClientConn = nil
	}

	c.rpcClientConnContext = nil
	c.rpcForwardingClient = nil

	clusterListener := c.getClusterListener()
	if clusterListener != nil {
		clusterListener.RemoveClient(consts.RequestForwardingALPN)
	}
	c.clusterLeaderParams.Store((*ClusterLeaderParams)(nil))
}

// ForwardRequest forwards a given request to the active node and returns the
// response.
func (c *Core) ForwardRequest(req *http.Request) (int, http.Header, []byte, error) {
	c.requestForwardingConnectionLock.RLock()
	defer c.requestForwardingConnectionLock.RUnlock()

	if c.rpcForwardingClient == nil {
		return 0, nil, nil, ErrCannotForward
	}

	defer metrics.MeasureSince([]string{"ha", "rpc", "client", "forward"}, time.Now())

	origPath := req.URL.Path
	defer func() {
		req.URL.Path = origPath
	}()

	req.URL.Path = req.Context().Value("original_request_path").(string)

	freq, err := forwarding.GenerateForwardedRequest(req)
	if err != nil {
		c.logger.Error("error creating forwarding RPC request", "error", err)
		return 0, nil, nil, fmt.Errorf("error creating forwarding RPC request")
	}
	if freq == nil {
		c.logger.Error("got nil forwarding RPC request")
		return 0, nil, nil, fmt.Errorf("got nil forwarding RPC request")
	}
	resp, err := c.rpcForwardingClient.ForwardRequest(req.Context(), freq)
	if err != nil {
		metrics.IncrCounter([]string{"ha", "rpc", "client", "forward", "errors"}, 1)
		c.logger.Error("error during forwarded RPC request", "error", err)
		return 0, nil, nil, fmt.Errorf("error during forwarding RPC request")
	}

	var header http.Header
	if resp.HeaderEntries != nil {
		header = make(http.Header)
		for k, v := range resp.HeaderEntries {
			header[k] = v.Values
		}
	}

	return int(resp.StatusCode), header, resp.Body, nil
}