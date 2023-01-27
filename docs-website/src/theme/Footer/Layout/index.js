import React from "react";
import clsx from "clsx";

// Reorder and remove default styling from copyright, logo, and links.
export default function FooterLayout({ style, links, logo, copyright }) {
  return (
    <footer
      className={clsx("footer", {
        "footer--dark": style === "dark",
      })}
    >
      <div className="container container-fluid">
        {logo}
        {copyright}
        {links}
      </div>
    </footer>
  );
}
