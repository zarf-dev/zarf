{% for sub_property in schema.iterate_properties %}
  {%- if sub_property.is_additional_properties and not sub_property.is_additional_properties_schema -%}
    {% continue %}
  {% endif %}

  {% set html_id = sub_property.html_id %}

  {% set description = sub_property | get_description %}

{% if sub_property.type_name == "object" or sub_property.type_name == "array" %}
<details open>
{% else %}
<details>
{% endif %}
<summary>
    {% filter md_heading(1, html_id, True) -%}
      {%- filter replace('\n', '') -%}
        {%- if sub_property is deprecated  -%}~~{%- endif -%}
        {%- if sub_property.is_pattern_property %} Pattern Property{% endif %} {% with schema=sub_property %}{%- include "breadcrumbs.md" %} {% endwith %}
        {%- if not skip_required and sub_property.property_name -%}
            {{ "*" if sub_property.is_required_property else "" -}}
        {%- endif -%}
        {%- if sub_property is deprecated -%}~~{%- endif -%}
      {%- endfilter %}
    {%- endfilter %}

</summary>
&nbsp;
<blockquote>

  {% if sub_property.type_name == "object" or sub_property.type_name == "array" %}
  ## {% with schema=sub_property %}
    {%- for node in schema.nodes_from_root -%}
      {%- if node.name_for_breadcrumbs == "root" or node.name_for_breadcrumbs.endswith(" items") -%}{% continue %}{%- endif -%}
      {{ node.name_for_breadcrumbs }}{%- if not loop.last %} > {% endif -%}
    {%- endfor -%}
  {% endwith %}
  {% endif %}

  {% with schema=sub_property, skip_headers=False %}
    {% if sub_property.is_pattern_property %}
:::note
All properties whose name matches the regular expression
```{{ sub_property.property_name }}``` ([Test](https://regex101.com/?regex={{ sub_property.property_name | urlencode }}))
must respect the following conditions
:::
    {% endif %}
    {%- if not skip_required and sub_property.property_name -%}
        {{ md_badge("Required", "red", show_text=False) if sub_property.is_required_property else "" -}}
    {%- endif -%}
    {% include "content.md" %}
  {% endwith %}

</blockquote>
</details>

{% endfor %}
