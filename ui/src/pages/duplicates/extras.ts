import type { Group } from "#/api/types.gen.ts";

/** A group is known by its first copy's path, which stays the same while the result does. */
export const groupKey = (g: Group) => g.copies?.[0]?.path ?? "";

/**
 * The copies to move to the Trash: for every picked group, all copies but the
 * one to keep (the first, unless the person chose another). Never all of a group.
 */
export function extras(groups: Group[], picked: ReadonlySet<string>, keep: ReadonlyMap<string, string>) {
  const paths: string[] = [];
  let bytes = 0;
  for (const g of groups) {
    const copies = g.copies ?? [];
    const key = groupKey(g);
    if (!picked.has(key) || copies.length < 2) continue;
    const kept = keep.get(key) ?? key;
    const keepIndex = Math.max(0, copies.findIndex((c) => c.path === kept));
    copies.forEach((c, i) => {
      if (i !== keepIndex) {
        paths.push(c.path);
        bytes += c.bytes;
      }
    });
  }
  return { paths, bytes };
}
