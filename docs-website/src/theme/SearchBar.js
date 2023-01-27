// By default, the classic theme does not provide any SearchBar implementation
// If you swizzled this, it is your responsibility to provide an implementation
// Tip: swizzle the SearchBar from the Algolia theme for inspiration:
// npm run swizzle @docusaurus/theme-search-algolia SearchBar
import SearchBar from "@easyops-cn/docusaurus-search-local/dist/client/client/theme/SearchBar";
import SearchSvg from "../css/images/svg/search-icon-dark.svg";
import React from "react";

const CustomSearchBar = (props) => {
  return (
    <div
      style={{ flexDirection: "row", display: "flex", alignItems: "center" }}
    >
      <SearchBar {...props}></SearchBar>
      <a className="svg-link mobile-search" href="/search">
        <SearchSvg />
      </a>
    </div>
  );
};
export default CustomSearchBar;
