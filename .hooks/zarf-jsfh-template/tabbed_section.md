<blockquote>

{{ current_node | md_array_items(title) | md_generate_table }}

{% for node in current_node.array_items %}
<blockquote>

    {% filter md_heading(depth+1, node.html_id) -%}
        {% if node.is_pattern_property %}Pattern{% endif %} Property `{% with schema=node %}{%- include "breadcrumbs.md" %}{% endwith %}`
    {%- endfilter %}

    {% with schema=node, skip_headers=False, depth=depth+1 %}
        {% include "content.md" %}
    {% endwith %}

</blockquote>
{% endfor %}

</blockquote>
