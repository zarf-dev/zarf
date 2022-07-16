:::note
Must be one of:
{% for enum_choice in schema.kw_enum.array_items %}
* {{ enum_choice.literal | python_to_json }}
{% endfor %}
:::
