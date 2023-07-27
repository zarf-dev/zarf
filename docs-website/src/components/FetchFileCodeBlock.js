import React from "react";
import { useEffect, useState } from "react";
import CodeBlock from "@theme/CodeBlock";

const FetchFileCodeBlock = ({ src, component, raw, showLink = true, fileFormat, fileName }) => {
  const [content, setContent] = useState(null);

  const linkBaseUrl = `${src}`.replace(/^\/build\/\.\.\//gm, '');

  useEffect(() => {
    //  when fileFormat is json then you don't need to fetch, it is [object Object]
    fileFormat !== "json" ?
      fetch(src)
        .then((res) => {
          return res.text()
        })
        .then(async (text) => {
          if (component) {
            const lines = text.split("\n");
            setContent(lines.join("\n"));
          } else {
            setContent(text);
          }
        }) : setContent(JSON.stringify(src, null, 2))
  }, []);

  if (!content) {
    // This is necessary so the bowser does not show [object Object] when fileFormat is json
    if (fileFormat === "json") {
      console.log(`Unable to fetch example ${fileFormat} ${fileName}`)
    } else {
      console.log(`Unable to fetch example ${fileFormat} ${src}`)
    }

    return <></>
  }
  if (raw) {
    return <>{content}</>;
  }
  return (
    <>
      {showLink && (
        <p>
          This example's full <code>{fileName}</code> can be viewed at{" "}
          {fileFormat === "json" ? <a download={fileName} href={"data:application/json;charset=utf-8,"+encodeURIComponent(JSON.stringify(src, null, 2))} style={{cursor: "pointer"}}>{fileName}</a> : <a href={`/${linkBaseUrl}/#zarf.yaml`}>{linkBaseUrl}</a>}
        </p>
      )}
      <CodeBlock copy={false} fileFormat={fileFormat}>
        {content}
      </CodeBlock>
    </>
  );


};

export default FetchFileCodeBlock;
