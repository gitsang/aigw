/**
 * 中文词条，按命名空间拆分。
 *
 * 每个命名空间文件对应一个顶层 key：`common.js` 对应 `t('common.action.save')`，
 * `nav.js` 对应 `t('nav.logs')`，依此类推。
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
