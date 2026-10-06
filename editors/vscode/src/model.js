'use strict';

// Tolerant source analysis for editing incomplete buffers. No code is executed.
// All offsets are UTF-16 offsets, matching VS Code's document APIs.
const catalog = require('./catalog.json');
const MAX_SOURCE_LENGTH = 256 * 1024;
const MAX_TAGS = 4096;
const MAX_NESTING = 64;
const WORD = /[\w$.-]/;
const STATES = ['hover', 'focus', 'pressed', 'disabled'];
const properties = Object.fromEntries(Object.entries(catalog.properties).flatMap(([name, entry]) =>
  [[name, { ...entry, name }], ...entry.aliases.map(alias => [alias, { ...entry, name: alias, canonical: name }])]));

function quotedEnd(text, start, end) {
  const quote = text[start];
  for (let i = start + 1; i < end; i++) {
    if (text[i] === '\\' && quote !== '`') { i++; continue; }
    if (text[i] === quote) return i + 1;
  }
  return end;
}

function goEnd(text, start, end) {
  let depth = 0;
  for (let i = start; i < end; i++) {
    if ('"\'`'.includes(text[i])) { i = quotedEnd(text, i, end) - 1; continue; }
    if (text.startsWith('//', i)) { const next = text.indexOf('\n', i); i = next < 0 ? end : next; continue; }
    if (text.startsWith('/*', i)) { const next = text.indexOf('*/', i + 2); i = next < 0 ? end : next + 1; continue; }
    if (text[i] === '{') depth++;
    if (text[i] === '}' && --depth === 0) return i + 1;
  }
  return end;
}

function insideGoLiteral(text, start, offset, end) {
  for (let i = start; i < offset; i++) {
    let next;
    if ('"\'`'.includes(text[i])) next = quotedEnd(text, i, end);
    if (text.startsWith('//', i)) {
      const close = text.indexOf('\n', i + 2);
      next = close < 0 ? end : close + 1;
    }
    if (text.startsWith('/*', i)) {
      const close = text.indexOf('*/', i + 2);
      next = close < 0 ? end : close + 2;
    }
    if (next !== undefined) {
      if (offset < next || next === end && offset === end) return true;
      i = next - 1;
    }
  }
  return false;
}

function readTag(text, start, limit) {
  const match = /^<(\/?)([\w-]*)/.exec(text.slice(start, limit));
  if (!match) return undefined;
  const nameStart = start + 1 + match[1].length;
  const tag = { name: match[2], start, nameStart, nameEnd: nameStart + match[2].length,
    closing: match[1] === '/', end: limit, complete: false, attributes: [] };
  let i = tag.nameEnd;
  while (i < limit) {
    if (/\s/.test(text[i])) { i++; continue; }
    if (text[i] === '>' || text.startsWith('/>', i)) {
      tag.selfClosing = text[i] === '/';
      tag.end = i + (tag.selfClosing ? 2 : 1); tag.complete = true;
      break;
    }
    const name = /^[\w-]+/.exec(text.slice(i, limit));
    if (!name) { i++; continue; }
    const attribute = { name: name[0], start: i, end: i + name[0].length };
    i = attribute.end;
    while (i < limit && /\s/.test(text[i])) i++;
    if (text[i] === '=') {
      attribute.equals = true; i++;
      while (i < limit && /\s/.test(text[i])) i++;
      attribute.valueStart = i;
      if ('"\''.includes(text[i] || '\0')) {
        const quote = text[i];
        attribute.valueStart = ++i;
        const close = text.indexOf(quote, i);
        attribute.valueEnd = close < 0 || close >= limit ? limit : close;
        attribute.value = text.slice(i, attribute.valueEnd);
        attribute.quoted = true;
        i = attribute.valueEnd < limit ? attribute.valueEnd + 1 : limit;
      } else if (text[i] === '{') {
        attribute.dynamic = true;
        i = goEnd(text, i, limit);
        attribute.valueEnd = i;
      } else {
        while (i < limit && !/[\s>]/.test(text[i])) i++;
        attribute.valueEnd = i;
        attribute.value = text.slice(attribute.valueStart, i);
      }
    }
    tag.attributes.push(attribute);
  }
  return tag;
}

function markupScan(text, start, end) {
  const tags = [], comments = [], expressions = [], stack = [];
  for (let i = start; i < end;) {
    if (text.startsWith('<!--', i)) {
      const close = text.indexOf('-->', i + 4);
      const next = close < 0 ? end : Math.min(end, close + 3);
      comments.push({ start: i, end: next }); i = next; continue;
    }
    if (text[i] === '{') {
      const lineStart = text.lastIndexOf('\n', i - 1) + 1;
      if (/^\s*(?:if\b|for\b|}\s*else\b|else\b)/.test(text.slice(lineStart, i))) { i++; continue; }
      const next = goEnd(text, i, end);
      expressions.push({ start: i, end: next }); i = next; continue;
    }
    if (i === start || text[i - 1] === '\n') {
      const header = /^[ \t\r]*(?:if|for)\b[^\n]*\{/.exec(text.slice(i, end));
      if (header) { i += header[0].length; continue; }
    }
    if (text[i] === '<') {
      const tag = readTag(text, i, end);
      if (tag) {
        if (tags.length === MAX_TAGS) break;
        tag.parents = stack.slice(); tags.push(tag);
        if (tag.complete) {
          if (tag.closing) {
            const index = stack.lastIndexOf(tag.name);
            if (index >= 0) stack.length = index;
          } else if (!tag.selfClosing && stack.length < MAX_NESTING) stack.push(tag.name);
        }
        i = tag.end; continue;
      }
    }
    i++;
  }
  return { tags, comments, expressions };
}

function maskStyle(text, start, end) {
  const chars = text.slice(start, end).split('');
  const comments = [];
  for (let i = 0; i < chars.length; i++) {
    if (chars[i] === '/' && (chars[i + 1] === '/' || chars[i + 1] === '*')) {
      const block = chars[i + 1] === '*';
      let close = block ? text.indexOf('*/', start + i + 2) : text.indexOf('\n', start + i + 2);
      close = close < 0 ? end : Math.min(end, close + (block ? 2 : 0));
      comments.push({ start: start + i, end: close, line: !block });
      for (; start + i < close; i++) if (chars[i] !== '\n' && chars[i] !== '\r') chars[i] = ' ';
      i--;
    }
  }
  return { masked: chars.join(''), comments };
}

function createModel(text) {
  if (text.length > MAX_SOURCE_LENGTH) return { text, regions: [], classes: [], variables: [] };
  const regions = [], classes = [], variables = [], outerComments = [];
  const withoutLeadingComments = text.replace(/^(?:\s|<!--[\s\S]*?-->)+/, '');
  const sfc = withoutLeadingComments.startsWith('<');
  if (sfc) {
    for (let i = 0; i < text.length;) {
      if (/\s/.test(text[i])) { i++; continue; }
      if (text.startsWith('<!--', i)) {
        const close = text.indexOf('-->', i + 4);
        const end = close < 0 ? text.length : close + 3;
        outerComments.push({ start: i, end, unclosed: close < 0 });
        i = end; continue;
      }
      const opening = readTag(text, i, text.length);
      if (!opening) break;
      if (!opening.complete || !['script', 'template', 'style'].includes(opening.name)) {
        regions.push({ kind: 'opening', start: i, end: opening.end, opening }); break;
      }
      const closing = new RegExp(`</${opening.name}\\s*>`, 'g');
      closing.lastIndex = opening.end;
      const match = closing.exec(text);
      regions.push({ kind: opening.name, start: opening.end, end: match ? match.index : text.length, opening });
      i = match ? closing.lastIndex : text.length;
    }
  } else {
    const declaration = /^[ \t]*(?:kartui|layout)\s+[^\n{]*\{/m.exec(text);
    const style = /^[ \t]*style[ \t]*\{/m.exec(text);
    if (declaration) {
      const start = declaration.index + declaration[0].length;
      regions.push({ kind: 'template', start, end: style ? style.index : text.length, legacy: true });
    }
    if (style) regions.push({ kind: 'style', start: style.index + style[0].length, end: text.length, legacy: true });
  }
  for (const region of regions) {
    if (region.kind === 'template') Object.assign(region, markupScan(text, region.start, region.end));
    if (region.kind !== 'style') continue;
    Object.assign(region, maskStyle(text, region.start, region.end));
    region.indented = !region.legacy && (region.opening.attributes.some(a => a.name === 'lang' && a.value === 'sass') || !region.masked.includes('{'));
    for (const match of region.masked.matchAll(/(?:^|[{};\n])[ \t\r]*\.([a-z][a-z0-9-]*)(?::(?:hover|focus|pressed|disabled))?\s*(?=\{|\r?$)/gm)) {
      const start = region.start + match.index + match[0].indexOf('.');
      classes.push({ name: match[1], start: start + 1, end: start + 1 + match[1].length });
    }
    for (const match of region.masked.matchAll(/^[ \t]*(\$[a-z][a-z0-9-]*):[ \t]*([^\n]*)/gm)) {
      const start = region.start + match.index + match[0].indexOf('$');
      variables.push({ name: match[1], value: match[2].trim(), start, end: start + match[1].length });
    }
  }
  return { text, regions, classes, variables, sfc, outerComments };
}

function wordRange(text, offset, pattern = WORD) {
  let start = offset, end = offset;
  while (start > 0 && pattern.test(text[start - 1])) start--;
  while (end < text.length && pattern.test(text[end])) end++;
  return { start, end, word: text.slice(start, end), prefix: text.slice(start, offset) };
}

function selectorContext(region, text, offset) {
  const before = region.masked.slice(0, offset - region.start);
  const lines = before.split('\n');
  const line = lines.pop();
  let selector = '', state = false, fragment = line;
  if (region.indented) {
    const stack = [];
    for (const raw of lines) {
      const trimmed = raw.trim();
      if (!trimmed || trimmed.startsWith('$') || /^[a-z][\w-]*\s*:/.test(trimmed) && !/^(?:panel|label|image|button|list|checkbox|input|slider|combo|tabs|tab):/.test(trimmed)) continue;
      const indent = raw.length - raw.trimStart().length;
      while (stack.length && stack.at(-1).indent >= indent) stack.pop();
      stack.push({ indent, selector: trimmed });
    }
    const indent = line.length - line.trimStart().length;
    while (stack.length && stack.at(-1).indent >= indent) stack.pop();
    selector = stack.findLast(entry => !entry.selector.startsWith('&') && !entry.selector.startsWith('@'))?.selector || '';
    state = stack.findLast(entry => /:(?:hover|focus|pressed|disabled)$/.test(entry.selector))?.selector.match(/:(hover|focus|pressed|disabled)$/)?.[1] || '';
  } else {
    const stack = [];
    let segment = '';
    for (const char of before) {
      if (char === '{') { stack.push(segment.trim()); segment = ''; }
      else if (char === '}') { stack.pop(); segment = ''; }
      else if (char === ';') segment = '';
      else segment += char;
    }
    selector = stack.findLast(entry => !entry.startsWith('@')) || '';
    state = selector.match(/:(hover|focus|pressed|disabled)$/)?.[1] || '';
    fragment = segment;
  }
  const declaration = /^\s*([a-z][\w-]*|\$[\w-]+)\s*:\s*([\s\S]*)$/.exec(fragment);
  if (declaration && properties[declaration[1]] || declaration?.[1].startsWith('$')) {
    return { type: 'styleValue', property: declaration[1], selector, state };
  }
  if (/^\s*&:[\w-]*$/.test(fragment)) return { type: 'state', selector };
  if (selector && !/^\s*[.@$]/.test(fragment)) return { type: 'property', selector, state };
  return { type: 'selector' };
}

function contextAt(model, offset) {
  if (model.text.length > MAX_SOURCE_LENGTH) return { type: 'none' };
  if (model.outerComments?.some(r => offset >= r.start && (offset < r.end || r.unclosed && offset === r.end))) return { type: 'none' };
  const region = model.regions.find(r => offset >= r.start && offset <= r.end);
  if (!region) {
    if (!model.sfc && model.text.trim()) return { type: 'none' };
    const start = model.text.lastIndexOf('<', offset - 1);
    const before = model.text.slice(start, offset);
    if (start >= 0 && /^<\/?[\w-]*$/.test(before) && !/<!--[^]*$/.test(before)) return { type: 'block', closing: before.startsWith('</') };
    return { type: 'none' };
  }
  if (region.kind === 'opening') return { type: 'block' };
  if (region.comments?.some(r => offset >= r.start && (offset < r.end || offset === r.end && (r.line || r.end === model.text.length)))) return { type: 'none' };
  const closingBlock = ['script', 'style'].includes(region.kind) && /^\s*<\/[\w-]*$/.test(model.text.slice(model.text.lastIndexOf('\n', offset - 1) + 1, offset));
  if (closingBlock && (region.kind !== 'script' || !insideGoLiteral(model.text, region.start, offset, region.end))) return { type: 'block', closing: true };
  if (region.kind === 'script') return { type: 'none' };
  if (region.kind === 'style') return { ...selectorContext(region, model.text, offset), region };
  if (region.expressions.some(r => offset >= r.start && offset < r.end)) return { type: 'none' };
  const tag = region.tags.find(t => offset > t.start && (offset < t.end || !t.complete && offset === t.end));
  if (!tag) return { type: 'none' };
  if (offset <= tag.nameEnd) return { type: 'tag', tag };
  if (tag.closing) return { type: 'none' };
  const attribute = tag.attributes.find(a => offset >= a.start && offset <= a.end || a.valueStart != null && offset >= a.valueStart && offset <= a.valueEnd);
  if (attribute?.valueStart != null && offset >= attribute.valueStart) {
    return { type: attribute.dynamic ? 'none' : 'attributeValue', tag, attribute };
  }
  return { type: 'attribute', tag, attribute };
}

function attributesFor(tag) {
  let names = catalog.tags[tag.name]?.attributes;
  if (!names && /^[A-Z]/.test(tag.name)) names = ['props', 'key', 'onBack'];
  if (tag.name === 'panel') names = tag.parents.length ? ['class', 'tooltip'] : ['class', 'modal', 'onBack'];
  return names || [];
}

function selectorTargets(model, selector) {
  const base = selector.split(':')[0];
  if (catalog.tags[base]) return base === 'panel' ? ['root', 'panel'] : [base];
  if (!base.startsWith('.')) return [];
  const kinds = new Set();
  for (const region of model.regions) {
    for (const tag of region.tags || []) {
      if (tag.attributes.some(a => a.name === 'class' && a.value === base.slice(1))) kinds.add(tag.name === 'panel' && !tag.parents.length ? 'root' : tag.name);
    }
  }
  return [...kinds];
}

function completions(model, offset) {
  const context = contextAt(model, offset);
  const range = wordRange(model.text, offset, context.type === 'styleValue' ? /[\w$%.-]/ : WORD);
  const item = (label, detail, insert = label, kind = 'Value', snippet = false) => ({ label, detail, insert, kind, snippet, start: range.start, end: range.end });
  let result = [];
  if (context.type === 'block') {
    result = ['script', 'template', 'style'].map(name => item(name, `SFC ${name} block.`, name === 'script' && !context.closing ? 'script setup lang="go"' : name, 'Keyword'));
  } else if (context.type === 'tag') {
    if (!context.tag.closing && context.tag.parents.length && !['panel', 'tab', 'tabs', 'slot'].includes(context.tag.parents.at(-1)) && !/^[A-Z]/.test(context.tag.parents.at(-1))) return [];
    const names = context.tag.closing ? context.tag.parents.slice().reverse() : !context.tag.parents.length ? ['panel'] : context.tag.parents.at(-1) === 'tabs' ? ['tab'] : Object.keys(catalog.tags).filter(name => name !== 'tab');
    result = [...new Set(names)].map(name => item(name, catalog.tags[name]?.detail || 'Close the enclosing tag.', name, 'Class'));
  } else if (context.type === 'attribute') {
    result = attributesFor(context.tag).filter(name => !context.tag.attributes.some(a => a.name === name && a !== context.attribute))
      .map(name => {
        const entry = catalog.attributes[name];
        let insert = entry.insert;
        const kind = context.tag.name;
        if (name === 'onChange') {
          const type = { checkbox: 'bool', input: 'string', slider: 'int32', combo: 'uint32', tabs: 'uint32' }[kind];
          insert = `onChange={func(v ${type}) { \${1} }}`;
        }
        if (name === 'onClick' && kind === 'list') insert = 'onClick={func(id uint32) { ${1} }}';
        return item(name, entry.detail, context.attribute?.equals ? name : insert, 'Property', !context.attribute?.equals);
      });
  } else if (context.type === 'attributeValue') {
    if (context.attribute.name === 'class' && /\s/.test(context.attribute.value || '')) return [];
    const values = context.attribute.name === 'class' ? model.classes.map(c => c.name) : catalog.attributes[context.attribute.name]?.values || [];
    result = [...new Set(values)].map(value => item(value, context.attribute.name === 'class' ? 'Style class defined in this file.' : catalog.attributes[context.attribute.name].detail));
  } else if (context.type === 'property') {
    const targets = selectorTargets(model, context.selector);
    result = Object.entries(properties).filter(([name, entry]) =>
      (!context.state || ['background', 'background-image', 'color'].includes(name)) &&
      (!targets.length || (context.state ? properties[name + '-' + context.state]?.targets : entry.targets)?.some(t => targets.includes(t))))
      .map(([name, entry]) => item(name, entry.detail, /^\s*:/.test(model.text.slice(range.end)) ? name : `${name}: `, 'Property'));
  } else if (context.type === 'styleValue') {
    const entry = properties[context.property];
    result = (entry?.values || []).map(value => item(value, entry.detail));
    result.push(...model.variables.filter(variable => variable.start < offset && variable.name !== context.property)
      .map(variable => item(variable.name, `Local style variable = ${variable.value}`, variable.name, 'Variable')));
  } else if (context.type === 'state') {
    const targets = selectorTargets(model, context.selector);
    result = STATES.filter(state => !targets.length || ['background', 'color', 'background-image'].some(name => properties[name + '-' + state].targets.some(t => targets.includes(t)))).map(state => item(state, 'State rules support background, background-image and color.', state, 'EnumMember'));
  } else if (context.type === 'selector') {
    result = Object.keys(catalog.tags).filter(name => !['slot', 'fragment'].includes(name)).map(name => item(name, catalog.tags[name].detail, name, 'Class'));
    result.push(...model.classes.map(c => item('.' + c.name, 'Style class defined in this file.', '.' + c.name, 'Class')));
  }
  return result.filter(entry => entry.label.startsWith(range.prefix));
}

function definitionAt(model, offset) {
  const context = contextAt(model, offset);
  const word = wordRange(model.text, offset).word;
  if (context.type === 'attributeValue' && context.attribute.name === 'class') return model.classes.filter(c => c.name === word);
  if (context.type === 'styleValue' && word.startsWith('$')) return model.variables.filter(v => v.name === word && v.start < offset).slice(-1);
  return [];
}

function hoverAt(model, offset) {
  const context = contextAt(model, offset);
  const range = wordRange(model.text, offset);
  let detail;
  if (context.type === 'tag') detail = catalog.tags[context.tag.name]?.detail;
  if (context.type === 'attribute') detail = catalog.attributes[context.attribute?.name]?.detail;
  if (context.type === 'property') detail = properties[range.word]?.detail;
  if (context.type === 'styleValue') detail = definitionAt(model, offset)[0]?.value;
  return detail ? { ...range, detail } : undefined;
}

module.exports = { createModel, contextAt, completions, hoverAt, definitionAt, MAX_SOURCE_LENGTH, MAX_TAGS, MAX_NESTING };
