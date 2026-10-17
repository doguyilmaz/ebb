/** Every concept the UI names, explained once. Tooltips, legends and the tour read from here. */
export interface Term {
  title: string;
  body: string;
}

const categories = {
  "package-cache": ["Package caches", "Packages that npm, bun, Gradle, CocoaPods and others keep so installs are faster. They download again when needed."],
  "build-artifact": ["Build output", "What builds leave behind: node_modules, DerivedData, target folders. The next build makes them again."],
  simulator: ["Simulators", "iOS simulators and their runtimes. Unused and unavailable ones can go; Xcode downloads runtimes again."],
  sdk: ["SDKs, emulators and Xcode", "Developer SDKs, emulator images and Xcode support files. Old versions tend to pile up."],
  "stale-project": ["Untouched projects", "Projects nobody changed for a while. Their dependencies and build output can be rebuilt later."],
  "app-cache": ["App caches", "Data apps keep in ~/Library/Caches to start faster. Apps rebuild it; the first launch may be slower."],
  logs: ["Logs", "Logs and crash reports. Only useful while debugging, and macOS keeps most of them forever."],
  installer: ["Installers", "Disk images and packages in Downloads. Once the app is installed they are not needed."],
  "large-file": ["Large files", "Single files over 1 GB in your own folders."],
  trash: ["Trash", "Already in the Trash, still taking space until it is emptied."],
  leftover: ["Leftovers of removed apps", "Data that apps left behind after they were deleted."],
  duplicate: ["Duplicates", "Identical files stored more than once."],
  snapshot: ["Time Machine", "Local Time Machine snapshots. macOS removes them on its own when space runs low."],
  growth: ["Recent growth", "What grew since the previous scan."],
  "user-data": ["Your data", "Your own files, shown so you know where space goes. ebb never removes them on its own."],
} as const;

const terms = {
  allocated: ["Allocated", "Space the files take on disk, counting every copy. This is what Finder and du show."],
  unique: ["Unique", "Space only these files use. Data shared with clones or hardlinks elsewhere is left out, so this is what deleting frees."],
  shared: ["Shared", "Blocks also used by an APFS clone or hardlink somewhere else. Deleting here does not free them."],
  reclaimable: ["Can be cleaned", "Everything ebb's findings can free, counted once even where findings overlap."],
  files: ["Files", "How many files are inside, counting every subfolder."],
  changed: ["Changed", "The newest change to anything inside."],
  "segment.system": ["macOS", "The system volume, swap, recovery and APFS's own reserve. Not something to clean by hand."],
  "segment.apps": ["Apps", "Everything in /Applications."],
  "segment.reclaimable": ["Can be cleaned", "Space ebb's findings can free. Each finding says why it is there and what removing it costs."],
  "segment.rest": ["Everything else", "Your files, projects and app data that no finding covers."],
  "segment.hidden": ["Not visible", "Used space the scan could not read: other users, apps' private data, system folders, local snapshots. Full Disk Access shows more of it."],
  "segment.free": ["Free", "Available now for new files."],
  purgeable: ["Purgeable", "Space macOS frees by itself when the disk fills up, mostly caches and iCloud copies."],
  "safety.regenerable": ["Rebuilds itself", "A cache or build output. The tool that made it downloads or rebuilds it when needed; the only cost is time."],
  "safety.caution": ["Check first", "Usually safe, but it may hold something you want, like an old download or an unused project. Look before cleaning."],
  "safety.user-data": ["Your data", "Documents, media or projects. ebb only removes these when you pick each one."],
  "safety.system": ["System", "Belongs to macOS. ebb shows it but never removes it."],
  "action.trash": ["Move to Trash", "The item goes to the Trash. Put it back from Activity until you empty the Trash."],
  "action.delete": ["Delete", "Removed right away without the Trash. Only for caches that rebuild themselves."],
  "action.command": ["Tool command", "Runs the tool's own cleanup command, so the tool stays consistent with its cache."],
  "action.empty-trash": ["Empty the Trash", "Frees what is already in the Trash. This cannot be undone."],
  "action.manual": ["Do it yourself", "ebb shows what to do; you do it in the app or tool that owns the data."],
  presets: ["Quick cleanups", "Ready-made groups of findings, like all developer caches. One click makes a plan you review before anything runs."],
  plan: ["Cleanup plan", "The exact list of what will happen. Nothing runs until you approve it, and each item is checked again right before."],
  ignore: ["Ignore", "Hides this finding, even after new scans, until you stop ignoring it. Nothing is removed."],
  protect: ["Protected paths", "Folders ebb will never clean, whatever a finding says."],
  note: ["Notes", "Your own reminder on a finding or folder, like why it stays. Kept on this Mac."],
  fda: ["Full Disk Access", "A macOS permission. Without it some folders (Mail, Safari, other apps' data) cannot be read and show as Not visible. ebb only reads; nothing leaves your Mac."],
  "view.sunburst": ["Sunburst", "Rings from the middle out, one ring per folder level. Wider slices are bigger. Click a slice to go in, the middle to go back."],
  "view.treemap": ["Treemap", "Each rectangle's area is its size, and folders hold their contents. Good for spotting single huge files."],
  "view.icicle": ["Icicle", "Columns by depth from left to right; each column splits its parent by size."],
  "view.list": ["List", "Every item in this folder, sortable, with what ebb found about it."],
  "color.folder": ["Colour by folder", "Each top folder gets its own colour, and everything inside it shares that hue, lighter as it goes deeper."],
  "color.age": ["Colour by age", "Light means changed recently, dark means untouched for a long time. Big dark areas are old data worth a look."],
  other: ["Other", "Many small items merged so the chart stays readable. The List shows them one by one."],
  small: ["Small files", "Files under 1 MB are counted together per folder, which keeps the scan fast."],
  ages: ["Age", "When files last changed. Data nothing touched for a year is often safe to archive or remove."],
  activity: ["Activity", "Everything ebb has cleaned. Trashed items can be put back until the Trash is emptied."],
  web: ["Web access", "Web pages that may talk to ebb on this Mac. A page needs your confirmation once before it can see anything."],
} as const;

type CategoryId = keyof typeof categories;
export type TermId = keyof typeof terms | `category.${CategoryId}`;

const entries = new Map<string, Term>([
  ...Object.entries(terms).map(([k, [title, body]]) => [k, { title, body }] as const),
  ...Object.entries(categories).map(([k, [title, body]]) => [`category.${k}`, { title, body }] as const),
]);

export function term(id: string): Term | undefined {
  return entries.get(id);
}

export function categoryTitle(id: string): string {
  return categories[id as CategoryId]?.[0] ?? id;
}
