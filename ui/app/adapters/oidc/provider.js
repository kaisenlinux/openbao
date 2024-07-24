/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import NamedPathAdapter from '../named-path';

export default class OidcProviderAdapter extends NamedPathAdapter {
  pathForType() {
    return 'identity/oidc/provider';
  }
}
