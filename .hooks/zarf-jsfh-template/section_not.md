{{ "Must **not** be" | md_heading(depth+1) }}
{% with schema=schema.kw_not, skip_headers=False, depth=depth+1, skip_required=True %}
    {% include "content.md" %}
{% endwith %}
