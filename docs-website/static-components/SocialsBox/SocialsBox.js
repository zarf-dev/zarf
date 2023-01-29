const fileToString = require("../utils/fileToString");

const SocialsValues = {
  SlackIconString: fileToString("static/img/SlackIcon.svg"),
  GithubIconString: fileToString("static/img/GithubIcon.svg"),
  SearchIconString: fileToString("src/css/images/svg/search-icon-dark.svg"),
  slackUrl: "https://zarf.dev/slack",
  githubUrl: "https://github.com/defenseunicorns/zarf",
};

function SocialsBox({ containerId = "", linkClass = "" } = {}) {
  return `
    <div id="${containerId}" class="socials-box">
        <a class="svg-link ${linkClass}" href="${SocialsValues.slackUrl}">
            ${SocialsValues.SlackIconString}
        </a>
        <a class="svg-link ${linkClass}" href="${SocialsValues.githubUrl}">
            ${SocialsValues.GithubIconString}
        </a>
    </div>
    <style>
      ${fileToString("static-components/SocialsBox/SocialsBox.css")}
    </style>
    `;
}

module.exports = {
  SocialsBox,
  SocialsValues,
};
