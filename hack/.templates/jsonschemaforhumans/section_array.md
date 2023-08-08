{% set min_items = schema.kw_min_items.literal or "N/A" %}
{% set max_items = schema.kw_max_items.literal or "N/A" %}
{% set kw_unique_items = schema.kw_unique_items.literal or "False" %}
{% set kw_additional_items = schema.kw_additional_items.literal or "N/A" %}

{{ md_badge("Min Items: " + min_items, "gold", show_text=False) }}
{{ md_badge("Max Items: " + max_items, "gold", show_text=False) }}
{{ md_badge("Item unicity: " + kw_unique_items, "gold", show_text=False) }}
{{ md_badge("Additional items: " + kw_additional_items, "gold", show_text=False) }}


{% if schema.array_items_def %} {% filter md_heading(2) %} {% with schema=schema.array_items_def %}{%- include "breadcrumbs.md" %}{% endwith %} {% endfilter %} {% with schema=schema.array_items_def, skip_headers=False, skip_required=True %} {% include "content.md" %} {% endwith %} {% endif %}

{% if schema.kw_items %}
{{ md_badge("Min Items: " + min_items, "gold", show_text=False) }}
{{ md_badge("Max Items: " + max_items, "gold", show_text=False) }}
{{ md_badge("Item unicity: " + kw_unique_items, "gold", show_text=False) }}
{{ md_badge("Additional items: " + kw_additional_items, "gold", show_text=False) }}


{% for item in schema.kw_items %}
    {% filter md_heading(3, item.html_id) %}
    {% with schema=item %}{%- include "breadcrumbs.md" %}{% endwith %}
    {% endfilter %}
    {% with schema=item, skip_headers=False, skip_required=True %}
        {% include "content.md" %}
    {% endwith %}
{% endfor %}
{% endif %}

{% if schema.kw_contains and schema.kw_contains.literal != {} %}
{{ "At least one of the items must be" | md_heading(3) }}
{% with schema=schema.kw_contains, skip_headers=False, skip_required=True %}
    {% include "content.md" %}
{% endwith %}
{% endif %}
