import React from "react";
import Translate from "@docusaurus/Translate";
import { ThemeClassNames } from "@docusaurus/theme-common";
import GithubIcon from "../../../static/img/GithubIcon.svg";
import ArrowForwardFilled from "../../css/images/svg/ArrowForwardFilled.svg";
export default function EditThisPage({ editUrl }) {
  return (
    <a
      href={editUrl}
      target="_blank"
      rel="noreferrer noopener"
      className={`${ThemeClassNames.common.editThisPage} svg-link`}
    >
      <GithubIcon />
      <Translate
        id="theme.common.editThisPage"
        description="The link label to edit the current page"
      >
        Edit this page
      </Translate>
      <ArrowForwardFilled />
    </a>
  );
}
