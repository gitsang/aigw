/**
 * English messages, assembled from one module per namespace.
 *
 * Every namespace file becomes a top-level key: `common.js` backs
 * `t('common.action.save')`, `nav.js` backs `t('nav.logs')`, and so on.
 */
import common from './common.js';
import nav from './nav.js';
import auth from './auth.js';
import app from './app.js';
import settings from './settings.js';
import providers from './providers.js';
import models from './models.js';
import logs from './logs.js';
import keys from './keys.js';
import virtualModels from './virtualModels.js';

export default {
  common,
  nav,
  auth,
  app,
  settings,
  providers,
  models,
  logs,
  keys,
  virtualModels
};
