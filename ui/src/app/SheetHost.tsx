import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { CleanupSheet } from "./CleanupSheet.tsx";

export function SheetHost() {
  const { request, clearSelection, close } = useSheet();
  const clear = useSelection((s) => s.clear);
  if (!request) return null;
  return <CleanupSheet key={JSON.stringify(request)} request={request} onClose={close} onDone={clearSelection ? clear : undefined} />;
}
