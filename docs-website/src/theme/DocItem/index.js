import React from "react";
import DocItem from "@theme-original/DocItem";
import Footer from "@theme-original/Footer";

export default function DocItemWrapper(props) {
  return (
    <>
      <DocItem {...props} />
      {/* place footer at bottom of doc item */}
      <Footer />
    </>
  );
}
