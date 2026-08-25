// ─── formspec.files — download tray (todo 5.9.4) ───
//
// Tracks downloaded files (e.g. Report export, Print) and exposes them to
// the renderer's download tray UI. `formspec.files` is injected into asset
// components as part of the formspec client.

export interface DownloadedFile {
  id: string
  name: string
  url: string
  size?: number
  addedAt: number
}

type Listener = (files: DownloadedFile[]) => void

const listeners = new Set<Listener>()
let fileList: DownloadedFile[] = []
let nextId = 1

function emit() {
  listeners.forEach((l) => l(fileList))
}

export function onFilesChange(listener: Listener): () => void {
  listeners.add(listener)
  listener(fileList)
  return () => listeners.delete(listener)
}

export function addDownloadedFile(
  name: string,
  url: string,
  size?: number,
): void {
  fileList = [
    { id: String(nextId++), name, url, size, addedAt: Date.now() },
    ...fileList,
  ]
  emit()
}

export function removeDownloadedFile(id: string): void {
  fileList = fileList.filter((f) => f.id !== id)
  emit()
}

export function clearDownloadedFiles(): void {
  fileList = []
  emit()
}

export function listDownloadedFiles(): DownloadedFile[] {
  return [...fileList]
}

/** The formspec.files surface injected into asset components. */
export const files = {
  download: (name: string, url: string, size?: number) =>
    addDownloadedFile(name, url, size),
  list: listDownloadedFiles,
  remove: removeDownloadedFile,
  clear: clearDownloadedFiles,
}
