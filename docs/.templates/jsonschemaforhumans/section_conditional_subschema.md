{% if schema.kw_if %}
    {% set first_property =  schema.kw_if | get_first_property %}

    {% if schema.kw_then %}
        {%- filter md_heading(2) -%}If (
            {{- first_property.property_name | md_escape_for_table -}}
            {{- " = " -}}
            {{- first_property.kw_const.literal | python_to_json -}}
        ){%- endfilter -%}
        {% with schema=schema.kw_then, skip_headers=False%}
            {% include "content.md" %}
        {% endwith %}
    {% endif %}
    {% if schema.kw_else %}
        {%- filter md_heading(2) -%}Else (i.e. {{ " " }}
            {{- first_property.property_name | md_escape_for_table -}}
            {{- " != " -}}
            {{- first_property.kw_const.literal | python_to_json -}}
        ){%- endfilter -%}
        {% with schema=schema.kw_else, skip_headers=False %}
            {% include "content.md" %}
        {% endwith %}
    {% endif %}
{% endif %}
