**Example{% if examples|length > 1 %}s{% endif %}:**{{ " " }}

<code>
{% for example in examples %}
{% set example_id = schema.html_id ~ "_ex" ~ loop.index %}
{{ example }}{%- if not loop.last %}, {% endif -%}
{% endfor %}
</code>
