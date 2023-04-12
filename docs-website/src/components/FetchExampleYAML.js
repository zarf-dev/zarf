import React from 'react';
import { useEffect, useState } from 'react';
import CodeBlock from "@theme/CodeBlock"
import { ZARF_REPO_RAW_URL } from './constants';

const FetchExampleYAML = ({ example, component, branch="main" }) => {
    const [content, setContent] = useState(null);
    const url = `${ZARF_REPO_RAW_URL}/${branch}/examples/${example}/zarf.yaml`

    useEffect(() => {
        fetch(url)
            .then((res) => res.text())
            .then(async (text) => {
                if (component) {
                    const lines = text.split('\n');
                    const start = lines.indexOf(`  - name: ${component}`);
                    const end = lines.findIndex((line, index) => index > start && line.startsWith('  - name: '));
                    setContent(lines.slice(start, end).join('\n'));
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
