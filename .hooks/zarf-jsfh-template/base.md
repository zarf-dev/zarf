{% set depth = 0 %}
{{ schema.keywords.get("title").literal | default("Zarf Package Schema") | md_heading(depth) }}
{% set contentBase %}
{% with schema=schema, skip_headers=False, depth=depth %}
    {% include "content.md" %}
{% endwith %}
{% endset %}

{{ md_get_toc() }}

{{ contentBase }}

----------------------------------------------------------------------------------------------------------------------------
{% if config.with_footer -%}
Generated from [zarf.schema.json](https://github.com/defenseunicorns/zarf/blob/master/zarf.schema.json){% if config.footer_show_time %} on {{ get_local_time() }}{% endif %}
{%- endif -%}

{{ "\n" }}
