const fs = require("fs");

/*
 * Docusaurus only allows us to inject string based html into the themed components.
 * This is necessary to include svg's and helps with the static components organization.
 */
function fileToString(filePath) {
  return fs.readFileSync(filePath).toString("utf-8");
}

module.exports = fileToString;
