import React from 'react';
import { useEffect, useState } from 'react';

const SchemaItemProperties = ({ item, include, invert}) => {
	const [itemSchema, setItemSchema] = useState(null);

	useEffect(() => {
		fetch('https://raw.githubusercontent.com/defenseunicorns/zarf/main/zarf.schema.json')
			.then((res) => res.json())
			.then((json) => {
				setItemSchema(json.definitions[item]);
			});
	}, []);

	if (!itemSchema) {
		return null;
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
							.filter((key) =>
								include ? (invert ? !include.includes(key) : include.includes(key)) : true
							)
							.sort()
							.sort((key) => itemSchema.required.includes(key) ? -1 : 1)
							.map((key) => {
								return (
									<tr key={key}>
										<td>
											<code>
												{key}
											</code>
											{itemSchema.required.includes(key) && <em>*</em>}
										</td>
										<td>
											<em>{itemSchema.properties[key].type ?? 'object'}</em>
										</td>
										<td>{itemSchema.properties[key].description}</td>
									</tr>
								);
							})}
				</tbody>
			</table>
			{itemSchema && itemSchema.required && itemSchema.required.some((ele) => include.includes(ele)) && (
				<small>
					<em>* Required field</em>
				</small>
			)}
		</>
	);
};

export default SchemaItemProperties;
