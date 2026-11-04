# Company logos

The Experience section shows a logo beside each company. To add one, drop a
square PNG named `<slug>.png` in this directory (about 128 px on a side). The
slug is the lowercase letters and digits of the company's first word, so
"Secuodsoft Technologies" uses `secuodsoft.png`.

Companies without a logo get a generated monogram: the first letter on a dark
tile. Logos are embedded at build time, so rebuild the TUI after adding one.
