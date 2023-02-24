{{ "Must **not** be" | md_heading(4) }}
{% with schema=schema.kw_not, skip_headers=False, skip_required=True %}
    {% include "content.md" %}
{% endwith %}
