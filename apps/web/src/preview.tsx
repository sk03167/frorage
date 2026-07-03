import type { FileRecord } from "@frorage/sdk";

export type PreviewState = {
  file: FileRecord;
  url: string;
  mimeType: string;
};

export const previewLimitBytes = 100 * 1024 * 1024;

export function PreviewContent({ preview }: { preview: PreviewState }) {
  if (preview.mimeType.startsWith("image/")) {
    return <img className="preview-media" src={preview.url} alt={preview.file.name} />;
  }
  if (preview.mimeType.startsWith("video/")) {
    return <video className="preview-media" src={preview.url} controls />;
  }
  if (preview.mimeType === "application/pdf") {
    return <iframe className="preview-frame" src={preview.url} title={preview.file.name} />;
  }
  return <p className="muted">Preview is not available for this file type.</p>;
}

export function canPreview(file: FileRecord): boolean {
  if (file.kind !== "file" || file.ciphertextSize > previewLimitBytes) return false;
  const mimeType = file.mimeType ?? "";
  return mimeType.startsWith("image/") || mimeType.startsWith("video/") || mimeType === "application/pdf";
}

export function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}
