import React from 'react';
import { useEffect, useState } from 'react';
import CodeBlock from "@theme/CodeBlock"

const FetchExampleYAML = ({ example, component, branch="main" }) => {
    const [content, setContent] = useState(null);
    const url = `https://raw.githubusercontent.com/defenseunicorns/zarf/${branch}/examples/${example}/zarf.yaml`

    useEffect(() => {
        fetch(url)
            .then((res) => res.text())
            .then(async (text) => {
                if (component) {
                    const yaml = await import("js-yaml");
                    let json = yaml.load(text);
                    const c = json.components.find((c) => c.name === component);
                    setContent(yaml.dump({components: [c]}).split("\n").slice(1).join("\n"));
                } else {
                    setContent(text);
                }
            });
    }, []);

    if (!content) {
        return <>Example YAML located <a href={url}>here</a>.</>;
    }
    return (
        <CodeBlock copy={false} title={`examples/${example}/zarf.yaml`} language="yaml">{content}</CodeBlock>
    );
}

export default FetchExampleYAML;
