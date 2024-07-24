// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pki

import (
	"context"
	"net/http"

	"github.com/openbao/openbao/sdk/framework"
	"github.com/openbao/openbao/sdk/logical"
)

func pathConfigCA(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/ca",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
			OperationVerb:   "configure",
			OperationSuffix: "ca",
		},

		Fields: map[string]*framework.FieldSchema{
			"pem_bundle": {
				Type: framework.TypeString,
				Description: `PEM-format, concatenated unencrypted
secret key and certificate.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathImportIssuers,
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"mapping": {
								Type:        framework.TypeMap,
								Description: "A mapping of issuer_id to key_id for all issuers included in this request",
								Required:    true,
							},
							"imported_keys": {
								Type:        framework.TypeCommaStringSlice,
								Description: "Net-new keys imported as a part of this request",
								Required:    true,
							},
							"imported_issuers": {
								Type:        framework.TypeCommaStringSlice,
								Description: "Net-new issuers imported as a part of this request",
								Required:    true,
							},
							"existing_keys": {
								Type:        framework.TypeCommaStringSlice,
								Description: "Existing keys specified as part of the import bundle of this request",
								Required:    true,
							},
							"existing_issuers": {
								Type:        framework.TypeCommaStringSlice,
								Description: "Existing issuers specified as part of the import bundle of this request",
								Required:    true,
							},
						},
					}},
				},
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigCAHelpSyn,
		HelpDescription: pathConfigCAHelpDesc,
	}
}

const pathConfigCAHelpSyn = `
Set the CA certificate and private key used for generated credentials.
`

const pathConfigCAHelpDesc = `
This sets the CA information used for credentials generated by this
by this mount. This must be a PEM-format, concatenated unencrypted
secret key and certificate.

For security reasons, the secret key cannot be retrieved later.
`

func pathConfigIssuers(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/issuers",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
		},

		Fields: map[string]*framework.FieldSchema{
			defaultRef: {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) to the default issuer.`,
			},
			"default_follows_latest_issuer": {
				Type:        framework.TypeBool,
				Description: `Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false.`,
				Default:     false,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersRead,
				DisplayAttrs: &framework.DisplayAttributes{
					OperationSuffix: "issuers-configuration",
				},
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"default": {
								Type:        framework.TypeString,
								Description: `Reference (name or identifier) to the default issuer.`,
								Required:    true,
							},
							"default_follows_latest_issuer": {
								Type:        framework.TypeBool,
								Description: `Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false.`,
								Required:    true,
							},
						},
					}},
				},
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersWrite,
				DisplayAttrs: &framework.DisplayAttributes{
					OperationVerb:   "configure",
					OperationSuffix: "issuers",
				},
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"default": {
								Type:        framework.TypeString,
								Description: `Reference (name or identifier) to the default issuer.`,
							},
							"default_follows_latest_issuer": {
								Type:        framework.TypeBool,
								Description: `Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false.`,
							},
						},
					}},
				},
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigIssuersHelpSyn,
		HelpDescription: pathConfigIssuersHelpDesc,
	}
}

func pathReplaceRoot(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "root/replace",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
			OperationVerb:   "replace",
			OperationSuffix: "root",
		},

		Fields: map[string]*framework.FieldSchema{
			"default": {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) to the default issuer.`,
				Default:     "next",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersWrite,
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"default": {
								Type:        framework.TypeString,
								Description: `Reference (name or identifier) to the default issuer.`,
								Required:    true,
							},
							"default_follows_latest_issuer": {
								Type:        framework.TypeBool,
								Description: `Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false.`,
								Required:    true,
							},
						},
					}},
				},
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigIssuersHelpSyn,
		HelpDescription: pathConfigIssuersHelpDesc,
	}
}

func (b *backend) pathCAIssuersRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot read defaults until migration has completed"), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)
	config, err := sc.getIssuersConfig()
	if err != nil {
		return logical.ErrorResponse("Error loading issuers configuration: " + err.Error()), nil
	}

	return b.formatCAIssuerConfigRead(config), nil
}

func (b *backend) formatCAIssuerConfigRead(config *issuerConfigEntry) *logical.Response {
	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef:                      config.DefaultIssuerId,
			"default_follows_latest_issuer": config.DefaultFollowsLatestIssuer,
		},
	}
}

func (b *backend) pathCAIssuersWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating issuers here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot update defaults until migration has completed"), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)

	// Validate the new default reference.
	newDefault := data.Get(defaultRef).(string)
	if len(newDefault) == 0 || newDefault == defaultRef {
		return logical.ErrorResponse("Invalid issuer specification; must be non-empty and can't be 'default'."), nil
	}
	parsedIssuer, err := sc.resolveIssuerReference(newDefault)
	if err != nil {
		return logical.ErrorResponse("Error resolving issuer reference: " + err.Error()), nil
	}
	entry, err := sc.fetchIssuerById(parsedIssuer)
	if err != nil {
		return logical.ErrorResponse("Unable to fetch issuer: " + err.Error()), nil
	}

	// Get the other new parameters. This doesn't exist on the /root/replace
	// variant of this call.
	var followIssuer bool
	followIssuersRaw, followOk := data.GetOk("default_follows_latest_issuer")
	if followOk {
		followIssuer = followIssuersRaw.(bool)
	}

	// Update the config
	config, err := sc.getIssuersConfig()
	if err != nil {
		return logical.ErrorResponse("Unable to fetch existing issuers configuration: " + err.Error()), nil
	}
	config.DefaultIssuerId = parsedIssuer
	if followOk {
		config.DefaultFollowsLatestIssuer = followIssuer
	}

	// Add our warning if necessary.
	response := b.formatCAIssuerConfigRead(config)
	if len(entry.KeyID) == 0 {
		msg := "This selected default issuer has no key associated with it. Some operations like issuing certificates and signing CRLs will be unavailable with the requested default issuer until a key is imported or the default issuer is changed."
		response.AddWarning(msg)
		b.Logger().Error(msg)
	}

	if err := sc.setIssuersConfig(config); err != nil {
		return logical.ErrorResponse("Error updating issuer configuration: " + err.Error()), nil
	}

	return response, nil
}

const pathConfigIssuersHelpSyn = `Read and set the default issuer certificate for signing.`

const pathConfigIssuersHelpDesc = `
This path allows configuration of issuer parameters.

Presently, the "default" parameter controls which issuer is the default,
accessible by the existing signing paths (/root/sign-intermediate,
/root/sign-self-issued, /sign-verbatim, /sign/:role, and /issue/:role).

The /root/replace path is aliased to this path, with default taking the
value of the issuer with the name "next", if it exists.
`

func pathConfigKeys(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/keys",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
		},

		Fields: map[string]*framework.FieldSchema{
			defaultRef: {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) of the default key.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathKeyDefaultWrite,
				DisplayAttrs: &framework.DisplayAttributes{
					OperationVerb:   "configure",
					OperationSuffix: "keys",
				},
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"default": {
								Type:        framework.TypeString,
								Description: `Reference (name or identifier) to the default issuer.`,
								Required:    true,
							},
						},
					}},
				},
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathKeyDefaultRead,
				DisplayAttrs: &framework.DisplayAttributes{
					OperationSuffix: "keys-configuration",
				},
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"default": {
								Type:        framework.TypeString,
								Description: `Reference (name or identifier) to the default issuer.`,
							},
						},
					}},
				},
				ForwardPerformanceStandby:   false,
				ForwardPerformanceSecondary: false,
			},
		},

		HelpSynopsis:    pathConfigKeysHelpSyn,
		HelpDescription: pathConfigKeysHelpDesc,
	}
}

func (b *backend) pathKeyDefaultRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot read key defaults until migration has completed"), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)
	config, err := sc.getKeysConfig()
	if err != nil {
		return logical.ErrorResponse("Error loading keys configuration: " + err.Error()), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef: config.DefaultKeyId,
		},
	}, nil
}

func (b *backend) pathKeyDefaultWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating keys here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot update key defaults until migration has completed"), nil
	}

	newDefault := data.Get(defaultRef).(string)
	if len(newDefault) == 0 || newDefault == defaultRef {
		return logical.ErrorResponse("Invalid key specification; must be non-empty and can't be 'default'."), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)
	parsedKey, err := sc.resolveKeyReference(newDefault)
	if err != nil {
		return logical.ErrorResponse("Error resolving issuer reference: " + err.Error()), nil
	}

	err = sc.updateDefaultKeyId(parsedKey)
	if err != nil {
		return logical.ErrorResponse("Error updating issuer configuration: " + err.Error()), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef: parsedKey,
		},
	}, nil
}

const pathConfigKeysHelpSyn = `Read and set the default key used for signing`

const pathConfigKeysHelpDesc = `
This path allows configuration of key parameters.

The "default" parameter controls which key is the default used by signing paths.
`