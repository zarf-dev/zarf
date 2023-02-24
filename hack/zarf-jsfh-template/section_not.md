{{ "Must **not** be" | md_heading(3) }}
{% with schema=schema.kw_not, skip_headers=False, skip_required=True %}
    {% include "content.md" %}
{% endwith %}
