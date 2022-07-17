{#
    content is a template and not a macro in md
        because macro parameters are not through context
        when rendering a template from the macro and it caused
        serious problems when using recursive calls
    mandatory context parameters:
    schema
#}
{# context parameters default values #}
{% set skip_headers = skip_headers or False %}
{% set depth = depth or 0 %}
{# end context parameters #}

{% set keys = schema.keywords %}
{%- if not skip_headers %}

{% if schema.title and schema.title | length > 0 %}
**Title:** {{ schema.title }}
{% endif %}

{% set description = (schema | get_description) %}
{% include "section_description.md" %}

{{ schema | md_type_info_table | md_generate_table }}

{% endif %}

{% if schema.should_be_a_link(config) %}
{% elif schema.refers_to -%}
    {%- with schema=schema.refers_to_merged, skip_headers=True, depth=depth -%}
        {% include "content.md" %}
    {% endwith %}
{% else %}

    {# Combining: allOf, anyOf, oneOf, not #}
    {% if schema.kw_all_of %}
        {% with operator="allOf", title="All of(Requirement)", current_node=schema.kw_all_of, skip_required=True %}
            {% include "tabbed_section.md" %}
        {% endwith %}
    {% endif %}
    {% if schema.kw_any_of %}
        {% with operator="anyOf", title="Any of(Option)", current_node=schema.kw_any_of, skip_required=True %}
            {% include "tabbed_section.md" %}
        {% endwith %}
    {% endif %}
    {% if schema.kw_one_of %}
        {% with operator="oneOf", title="One of(Option)",current_node=schema.kw_one_of, skip_required=True %}
            {% include "tabbed_section.md" %}
        {% endwith %}
    {% endif %}
    {% if schema.kw_not %}
        {% include "section_not.md" %}
    {% endif %}

    {# Enum and const #}
    {% if schema.kw_enum -%}
        {% include "section_one_of.md" %}
    {%- endif %}
    {%- if schema.kw_const -%}
        Specific value: `{{ schema.kw_const.raw | python_to_json }}`
    {%- endif -%}

    {# Conditional subschema, or if-then-else section #}
    {% if schema.has_conditional %}
        {% with skip_headers=False, depth=depth+1 %}
            {% include "section_conditional_subschema.md" %}
        {% endwith %}
    {% endif %}

    {# Required properties that are not defined under "properties". They will only be listed #}
    {% include "section_undocumented_required_properties.md" %}

    {# Show the requested type(s) #}
    {{- schema | md_restrictions_table | md_generate_table -}}

    {# Show array restrictions #}
    {% if schema.type_name.startswith("array") %}
        {% include "section_array.md" %}
    {% endif %}

    {# Display examples #}
    {% set examples = schema.examples %}
    {% if examples %}
        {% include "section_examples.md" %}
    {% endif %}

    {# details of Properties, pattern properties, additional properties #}
    {% if schema.type_name == "object" %}
    {% with skip_required=False %}
        {% include "section_properties_details.md" %}
    {% endwith %}
    {% endif %}
{% endif %}
