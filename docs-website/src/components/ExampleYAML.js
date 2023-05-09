import React from "react";
import { useEffect, useState } from "react";
import CodeBlock from "@theme/CodeBlock";

const FetchExampleYAML = ({ example, component, raw, showLink = true }) => {
  const [content, setContent] = useState(null);
  const url = `/${example}/zarf.yaml`;

  useEffect(() => {
    fetch(url)
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
          <a href={`/examples/${example}/?view=zarf.yaml`}>examples/{example}</a>
        </p>
      )}
      <CodeBlock copy={false} title={`examples/${example}/zarf.yaml`} language="yaml">
        {content}
      </CodeBlock>
    </>
  );
};

export default FetchExampleYAML;
