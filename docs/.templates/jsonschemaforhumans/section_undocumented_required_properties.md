{% set undocumented_required_properties = schema | get_undocumented_required_properties %}
{% if undocumented_required_properties%}
{{ "The following properties are required" | md_heading(3) }}
{% for required_property in undocumented_required_properties %}
* {{ required_property }}
{% endfor %}
{% endif %}
