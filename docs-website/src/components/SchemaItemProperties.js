import React from "react";
import { useEffect, useState } from "react";

const SchemaItemProperties = ({ item, include, invert }) => {
  const [itemSchema, setItemSchema] = useState(null);

  useEffect(async () => {
    const json = await import("@site/static/zarf.schema.json");
    setItemSchema(json.definitions[item]);
  }, []);

  if (!itemSchema) {
    return null;
  }

  let includesElement = (key) => {
    if (!include) {
      return true;
    } else if (invert) {
      return !include.includes(key);
    } else {
      return include.includes(key);
    }
  }

  return (
    <>
      <table>
        <thead>
          <tr>
            <th>Field</th>
            <th>Type</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          {itemSchema &&
            Object.keys(itemSchema.properties)
              .filter(includesElement)
              .sort()
              .sort((key) => (itemSchema.required.includes(key) ? -1 : 1))
              .map((key) => {
                return (
                  <tr key={key}>
                    <td>
                      <code>{key}</code>
                      {itemSchema.required.includes(key) && <em>*</em>}
                    </td>
                    <td>
                      <em>{itemSchema.properties[key].type ?? "object"}</em>
                    </td>
                    <td>{itemSchema.properties[key].description}</td>
                  </tr>
                );
              })}
        </tbody>
      </table>
      {itemSchema && itemSchema.required && itemSchema.required.some(includesElement) && (
        <small>
          <em>* Required field</em>
          <br/>&nbsp;
        </small>
      )}
    </>
  );
};

export default SchemaItemProperties;
