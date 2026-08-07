"""Small HTML-rendering helpers shared by report and comparison output.

compare.py depends on this rather than reaching into report.py's private
names, keeping the data/comparison layer from having a presentation-module
dependency.
"""

import html


def flatten(value, prefix=""):
    if isinstance(value, dict):
        for key, child in value.items():
            name = f"{prefix}.{key}" if prefix else str(key)
            yield from flatten(child, name)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from flatten(child, f"{prefix}[{index}]")
    else:
        yield prefix, value


def display_value(value) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return str(value).lower()
    return str(value)


def field_label(label: str, description: str) -> str:
    escaped_label = html.escape(label)
    escaped_description = html.escape(description, quote=True)
    return (
        f'<span class="field-label">{escaped_label}'
        f'<button type="button" class="field-help" data-tooltip="{escaped_description}" '
        f'aria-label="About {escaped_label}: {escaped_description}">i</button></span>'
    )
