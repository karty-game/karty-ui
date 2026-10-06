'use strict';

const language = require('./model');

function register(vscode, context) {
  const cache = new WeakMap();
  const modelFor = document => {
    const previous = cache.get(document);
    if (previous?.version === document.version) return previous.model;
    const model = language.createModel(document.getText());
    cache.set(document, { version: document.version, model });
    return model;
  };
  const rangeFor = (document, entry) => new vscode.Range(document.positionAt(entry.start), document.positionAt(entry.end));
  const markdown = detail => {
    const content = new vscode.MarkdownString();
    content.appendText(detail); // Authored variable text cannot become trusted Markdown/commands.
    return content;
  };
  const selector = { language: 'kartui' };
  context.subscriptions.push(vscode.languages.registerCompletionItemProvider(selector, {
    provideCompletionItems(document, position, token) {
      if (token.isCancellationRequested) return [];
      return language.completions(modelFor(document), document.offsetAt(position)).map(entry => {
        const item = new vscode.CompletionItem(entry.label, vscode.CompletionItemKind[entry.kind]);
        item.range = rangeFor(document, entry);
        item.insertText = entry.snippet ? new vscode.SnippetString(entry.insert) : entry.insert;
        item.documentation = markdown(entry.detail);
        item.detail = 'KartUI';
        return item;
      });
    },
  }, '<', '/', ' ', ':', '.', '$', '"', "'"));
  context.subscriptions.push(vscode.languages.registerHoverProvider(selector, {
    provideHover(document, position, token) {
      if (token.isCancellationRequested) return undefined;
      const entry = language.hoverAt(modelFor(document), document.offsetAt(position));
      return entry ? new vscode.Hover(markdown(entry.detail), rangeFor(document, entry)) : undefined;
    },
  }));
  context.subscriptions.push(vscode.languages.registerDefinitionProvider(selector, {
    provideDefinition(document, position, token) {
      if (token.isCancellationRequested) return [];
      return language.definitionAt(modelFor(document), document.offsetAt(position)).map(entry => new vscode.Location(document.uri, rangeFor(document, entry)));
    },
  }));
}

module.exports = { register };
