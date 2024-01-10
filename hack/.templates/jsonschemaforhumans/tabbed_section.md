<blockquote>

{{ current_node | md_array_items(title) | md_generate_table }}

{% for node in current_node.array_items %}
<blockquote>

    {% with schema=node, skip_headers=False %}
        {% include "content.md" %}
    {% endwith %}

</blockquote>
{% endfor %}

</blockquote>
