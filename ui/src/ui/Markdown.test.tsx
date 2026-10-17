import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { Markdown } from "./Markdown.tsx";

test("markdown renders structure and never raw HTML", () => {
  const html = renderToStaticMarkup(<Markdown text={"**Bun** cache\n\n- one `npm`\n- two\n\n<img src=x onerror=alert(1)>"} />);
  expect(html).toContain("<strong>Bun</strong>");
  expect(html).toContain("<li>one <code>npm</code></li>");
  expect(html).not.toContain("<img");
  expect(html).toContain("&lt;img");
});
