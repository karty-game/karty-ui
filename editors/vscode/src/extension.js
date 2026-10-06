"use strict";

const vscode = require("vscode");
const { register } = require("./providers");

function activate(context) {
  register(vscode, context);
}
module.exports = { activate };
