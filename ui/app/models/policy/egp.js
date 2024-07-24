/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { attr } from '@ember-data/model';
import { computed } from '@ember/object';

import PolicyModel from './rgp';
import { expandAttributeMeta } from 'vault/utils/field-to-attrs';

export default PolicyModel.extend({
  paths: attr({
    editType: 'stringArray',
  }),
  additionalAttrs: computed(function () {
    return expandAttributeMeta(this, ['enforcementLevel', 'paths']);
  }),
});