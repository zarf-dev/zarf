import React from "react";
import { useEffect, useState } from "react";
import CodeBlock from "@theme/CodeBlock";

const FetchExampleYAML = ({ src, component, raw, showLink = true }) => {
  const [content, setContent] = useState(null);
  const linkBaseUrl = `${src}`.replace(/^\/build\/\.\.\//gm,'').replace(/\/zarf\.yaml.+?$/gm,'');

  useEffect(() => {
    fetch(src)
      .then((res) => res.text())
      .then(async (text) => {
        if (component) {
          const lines = text.split("\n");
          const start = lines.indexOf(`  - name: ${component}`);
          const end = lines.findIndex((line, index) => index > start && line.startsWith("  - name: "));
          setContent(lines.slice(start, end).join("\n"));
        } else {
          setContent(text);
        }
      });
  }, []);

  if (!content) {
    console.log(`Unable to fetch example YAML ${src}`)
    return <></>
  }
  if (raw) {
    return <>{content}</>;
  }
  return (
    <>
      {showLink && (
        <p>
          This example's full <code>zarf.yaml</code> can be viewed at{" "}
          <a href={`/${linkBaseUrl}/#zarf.yaml`}>{linkBaseUrl}</a>
        </p>
      )}
      <CodeBlock copy={false} language="yaml">
        {content}
      </CodeBlock>
    </>
  );
};

export default FetchExampleYAML;
