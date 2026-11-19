import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { HvigorPlugin } from '@ohos/hvigor';
import { hapTasks } from '@ohos/hvigor-ohos-plugin';

const buildInfo: HvigorPlugin = {
  pluginId: 'cmr-build-info',
  apply(node) {
    const moduleDir = node.getNodePath();
    const manifest = readFileSync(join(moduleDir, '../AppScope/app.json5'), 'utf8');
    const version = /"versionName"\s*:\s*"([^"]+)"/.exec(manifest)?.[1];
    const build = /"versionCode"\s*:\s*(\d+)/.exec(manifest)?.[1];
    if (!version || !build) throw new Error('Harmony versionName or versionCode is missing');

    const now = new Date();
    const pad = (value: number) => String(value).padStart(2, '0');
    const time = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
    const directory = join(moduleDir, 'src/main/ets/common');
    mkdirSync(directory, { recursive: true });
    writeFileSync(join(directory, 'BuildInfo.ets'), `export const APP_BUILD_LABEL: string = ${JSON.stringify(`版本 ${version} · 构建号 ${build} · 构建时间 ${time}`)};\n`);
  },
};

export default {
  system: hapTasks,
  plugins: [buildInfo],
};
