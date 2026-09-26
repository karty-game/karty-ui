import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import test from 'node:test';
import textmate from 'vscode-textmate';
import oniguruma from 'vscode-oniguruma';

const require = createRequire(import.meta.url);
const wasm = await readFile(require.resolve('vscode-oniguruma/release/onig.wasm'));
await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
const source = await readFile(new URL('../syntaxes/kartui.tmLanguage.json', import.meta.url), 'utf8');
const registry = new textmate.Registry({
  onigLib: Promise.resolve({ createOnigScanner: p => new oniguruma.OnigScanner(p), createOnigString: s => new oniguruma.OnigString(s) }),
  loadGrammar: async scope => scope === 'source.kartui' ? textmate.parseRawGrammar(source, 'kartui.json') : null,
});
const grammar = await registry.loadGrammar('source.kartui');

function tokenize(source) {
  let state = textmate.INITIAL;
  const result = [];
  for (const line of source.split('\n')) {
    const next = grammar.tokenizeLine(line, state);
    result.push(...next.tokens.map(t => ({ text: line.slice(t.startIndex, t.endIndex), scopes: t.scopes })));
    state = next.ruleStack;
  }
  return result;
}
function has(tokens, text, scope) {
  assert.ok(tokens.some(t => t.text === text && t.scopes.includes(scope)), `${JSON.stringify(text)} missing ${scope}`);
}

test('actual demo declaration, parameters, markup and bindings', async () => {
  const demo = await readFile(new URL('./fixtures/menu.ui', import.meta.url), 'utf8');
  const tokens = tokenize(demo);
  has(tokens, 'kartui', 'keyword.declaration.kartui');
  has(tokens, 'Menu', 'entity.name.function.kartui');
  has(tokens, 'AppProps', 'variable.other.go');
  has(tokens, 'setup', 'keyword.declaration.kartui');
  has(tokens, 'MainMenuLayout', 'entity.name.tag.html');
  has(tokens, 'onClick', 'entity.other.attribute-name.html');
  has(tokens, 'icon', 'support.type.property-name.css');
  has(tokens, 'choose', 'meta.embedded.expression.go');
});

test('nested Go braces, comments, escaped strings and raw strings do not swallow tags', () => {
  const tokens = tokenize('kartui Menu(Click func()) {\n<panel modal="true">\n' +
    '<button onClick={ func() { /* } > */ if true { Click() } } }>Click</button>\n' +
    '<label>{ "quoted \\\" } >" + `raw }\n >` }</label>\n' +
    '<!-- { <fake> } -->\n<label>Done</label>\n</panel>\n}\n' +
    'kartui Next() { <panel><label>Next</label></panel> }');
  has(tokens, 'button', 'entity.name.tag.html');
  has(tokens, 'label', 'entity.name.tag.html');
  has(tokens, 'Next', 'entity.name.function.kartui');
  assert.ok(tokens.some(t => t.scopes.includes('string.quoted.raw.go')));
  assert.ok(!tokens.some(t => t.text === 'fake' && t.scopes.includes('entity.name.tag.html')));
  const done = tokens.find(t => t.text === 'Done');
  assert.ok(done && !done.scopes.includes('meta.embedded.expression.go'));
});

test('manifest and snippet contributions remain declarative', async () => {
  const manifest = JSON.parse(await readFile(new URL('../package.json', import.meta.url)));
  assert.equal(manifest.main, undefined);
  assert.equal(manifest.browser, undefined);
  assert.equal(manifest.contributes.languages[0].id, 'kartui');
  const snippets = JSON.parse(await readFile(new URL('../snippets/kartui.json', import.meta.url)));
  assert.ok(snippets.Component.body.some(line => line.startsWith('kartui ')));
  JSON.parse(await readFile(new URL('../language-configuration.json', import.meta.url)));
});

test('keyed component loop keeps markup highlighted', async () => {
  const source = await readFile(new URL('./fixtures/inventory.ui', import.meta.url), 'utf8');
  const tokens = tokenize(source);
  has(tokens, 'for', 'keyword.control.go');
  has(tokens, 'ItemRow', 'entity.name.tag.html');
  has(tokens, 'key', 'entity.other.attribute-name.html');
  has(tokens, 'props', 'entity.other.attribute-name.html');
  has(tokens, 'onBack', 'entity.other.attribute-name.html');
});

test('style blocks highlight selectors, properties and theme tokens', () => {
  const tokens = tokenize('kartui Menu { <panel class="screen"/> }\nstyle {\n.screen:hover { background-image: theme.images.panel; image-fit: contain; flex-direction: row; align-items: center; justify-content: space-between; overflow: scroll; text-align: left; font-family: display; }\n}');
  has(tokens, 'style', 'keyword.declaration.kartui');
  has(tokens, '.screen:hover', 'entity.other.attribute-name.class.css');
  has(tokens, 'background-image', 'support.type.property-name.css');
  has(tokens, 'theme.images.panel', 'variable.other.constant.css');
  has(tokens, 'flex-direction', 'support.type.property-name.css');
  has(tokens, 'row', 'support.constant.property-value.css');
  has(tokens, 'center', 'support.constant.property-value.css');
  has(tokens, 'space-between', 'support.constant.property-value.css');
  has(tokens, 'scroll', 'support.constant.property-value.css');
  has(tokens, 'text-align', 'support.type.property-name.css');
  has(tokens, 'left', 'support.constant.property-value.css');
  has(tokens, 'font-family', 'support.type.property-name.css');
  has(tokens, 'display', 'support.constant.property-value.css');
  has(tokens, 'contain', 'support.constant.property-value.css');
});

test('layout declarations and slots are highlighted', async () => {
  const source = await readFile(new URL('./fixtures/window.ui', import.meta.url), 'utf8');
  const tokens = tokenize(source);
  has(tokens, 'layout', 'keyword.declaration.kartui');
  has(tokens, 'WindowLayout', 'entity.name.type.kartui');
  has(tokens, 'slot', 'entity.name.tag.html');
  has(tokens, '@media', 'keyword.control.at-rule.media.css');
  has(tokens, 'max-width', 'support.type.property-name.media.css');
});

test('conditional markup keeps Go conditions and both branches highlighted', () => {
  const tokens = tokenize('kartui Empty { <panel>\nif len(items) == 0 {\n<label>Empty</label>\n} else {\n<button>Use</button>\n}\n</panel> }');
  has(tokens, 'if', 'keyword.control.go');
  has(tokens, 'else', 'keyword.control.go');
  has(tokens, 'label', 'entity.name.tag.html');
  has(tokens, 'button', 'entity.name.tag.html');
});
