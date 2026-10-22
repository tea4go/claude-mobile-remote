// Standard TypeScript checking of platform-independent ArkTS logic. Not a DevEco build.
const ts = require('../../mobile/node_modules/typescript')
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const root = path.resolve(__dirname, '../entry/src/main/ets')
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'cmr-harmony-check-'))
try {
  const files = []
  for (const directory of ['protocol', 'session', 'debug']) {
    fs.mkdirSync(path.join(temporary, directory))
    for (const file of fs.readdirSync(path.join(root, directory)).filter(file => file.endsWith('.ets'))) {
      const destination = path.join(temporary, directory, file.replace('.ets', '.ts'))
      fs.writeFileSync(destination, fs.readFileSync(path.join(root, directory, file))); files.push(destination)
    }
  }
  const platform = path.join(temporary, 'platform.d.ts')
  fs.writeFileSync(platform, `declare module '@kit.BasicServicesKit' { export interface BusinessError extends Error { code: number } }
  declare module '@kit.NetworkKit' { export namespace webSocket {
    interface CloseResult { code: number; reason: string }
    interface WebSocket {
      on(event: 'open', callback: (error: import('@kit.BasicServicesKit').BusinessError, data: Object) => void): void;
      on(event: 'message', callback: (error: import('@kit.BasicServicesKit').BusinessError, data: string | ArrayBuffer) => void): void;
      on(event: 'close', callback: (error: import('@kit.BasicServicesKit').BusinessError, data: CloseResult) => void): void;
      on(event: 'error', callback: (error: import('@kit.BasicServicesKit').BusinessError) => void): void;
      connect(url: string, callback: (error: import('@kit.BasicServicesKit').BusinessError, result: boolean) => void): void;
      send(data: string): Promise<boolean>;
      close(): Promise<boolean>;
    }
    function createWebSocket(): WebSocket;
  } }`)
  files.push(platform)
  const program = ts.createProgram(files, { strict: true, noEmit: true, target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler, skipLibCheck: true })
  const diagnostics = ts.getPreEmitDiagnostics(program)
  if (diagnostics.length) {
    console.error(ts.formatDiagnosticsWithColorAndContext(diagnostics, { getCanonicalFileName: file => file, getCurrentDirectory: () => temporary, getNewLine: () => '\n' }))
    process.exitCode = 1
  } else console.log(`TypeScript logic check: ${files.length - 1} files passed; platform declarations are test adapters, not SDK validation.`)
  // Native source syntax only, without claiming platform signatures or ArkUI compilation.
  for (const file of fs.readdirSync(path.join(root, 'native')).filter(file => file.endsWith('.ets'))) {
    const result = ts.transpileModule(fs.readFileSync(path.join(root, 'native', file), 'utf8'), { fileName: file.replace('.ets', '.ts'), reportDiagnostics: true,
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
    const errors = result.diagnostics?.filter(item => item.category === ts.DiagnosticCategory.Error) ?? []
    if (errors.length) { console.error(file, errors.map(item => item.messageText)); process.exitCode = 1 }
  }
} finally { fs.rmSync(temporary, { recursive: true, force: true }) }
