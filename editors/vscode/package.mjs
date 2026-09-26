// Build-owned VSIX packaging. Source files remain editable; staging is temporary.
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = dirname(fileURLToPath(import.meta.url));
const manifest = JSON.parse(await readFile(join(root, 'package.json')));
const output = resolve(root, '../../dist/editor', `kartui-${manifest.version}.vsix`);
const staging = await mkdtemp(join(tmpdir(), 'karty-vsix-'));
const warning = `<!-- GENERATED FILE — DO NOT EDIT.
     GENERATED FILE — DO NOT EDIT.
     Source: editors/vscode/package.mjs. Regenerate with: mise run package-editor. -->`;
try {
  await mkdir(join(staging, 'extension'));
  for (const file of ['package.json', 'README.md', 'language-configuration.json', 'syntaxes', 'snippets']) {
    await cp(join(root, file), join(staging, 'extension', file), { recursive: true });
  }
  await cp(resolve(root, '../../LICENSE.md'), join(staging, 'extension', 'LICENSE.md'));
  await writeFile(join(staging, '[Content_Types].xml'), `<?xml version="1.0" encoding="utf-8"?>\n${warning}
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="json" ContentType="application/json"/><Default Extension="md" ContentType="text/markdown"/><Default Extension="vsixmanifest" ContentType="text/xml"/></Types>`);
  await writeFile(join(staging, 'extension.vsixmanifest'), `<?xml version="1.0" encoding="utf-8"?>\n${warning}
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011"><Metadata>
<Identity Language="en-US" Id="kartui" Version="${manifest.version}" Publisher="karty-local"/>
<DisplayName>KartUI</DisplayName><Description xml:space="preserve">KartUI syntax highlighting and snippets</Description>
<Properties><Property Id="Microsoft.VisualStudio.Code.Engine" Value="^1.85.0"/><Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="ui"/></Properties>
</Metadata><Installation><InstallationTarget Id="Microsoft.VisualStudio.Code"/></Installation><Dependencies/>
<Assets><Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true"/></Assets></PackageManifest>`);
  await mkdir(dirname(output), { recursive: true });
  // zip writes a fresh archive within this invocation's private staging folder.
  execFileSync('zip', ['-q', '-r', 'kartui.vsix', 'extension', '[Content_Types].xml', 'extension.vsixmanifest'], { cwd: staging });
  await cp(join(staging, 'kartui.vsix'), output);
  console.log(output);
} finally {
  await rm(staging, { recursive: true, force: true });
}
