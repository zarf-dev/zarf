{% filter md_escape_for_table %}
{%- if config.show_breadcrumbs %}
  {%- for node in schema.nodes_from_root -%}
    {{ node.name_for_breadcrumbs }}{%- if not loop.last %} > {% endif -%}
  {%- endfor -%}
{%- else -%}
  {{- schema.name_for_breadcrumbs or schema.property_name -}}
{% endif %}
{% endfilter %}
