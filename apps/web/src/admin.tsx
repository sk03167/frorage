import React, { useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { Download, Eye, FolderOpen, ShieldCheck, X } from "lucide-react";
import { PrivateCloudClient, type AdminUser, type DownloadedFile, type FileRecord } from "@frorage/sdk";
import { bytesToArrayBuffer, canPreview, PreviewContent, type PreviewState } from "./preview";
import "./styles.css";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? window.location.origin;

function AdminApp() {
  const client = useMemo(() => new PrivateCloudClient({ baseUrl: apiBaseUrl }), []);
  const [emailSearch, setEmailSearch] = useState("");
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null);
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [status, setStatus] = useState("Admin recovery");
  const visibleFiles = files.filter((file) => (file.parentId ?? null) === currentFolderId);

  function closePreview() {
    setPreview((current) => {
      if (current) URL.revokeObjectURL(current.url);
      return null;
    });
  }

  async function searchUsers(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus("Searching users...");
    const nextUsers = await client.adminUsers(emailSearch);
    setUsers(nextUsers);
    setSelectedUser(null);
    setFiles([]);
    setCurrentFolderId(null);
    setStatus(nextUsers.length === 0 ? "No users found." : `${nextUsers.length} user found.`);
  }

  async function openUser(user: AdminUser) {
    setSelectedUser(user);
    setCurrentFolderId(null);
    closePreview();
    setStatus("Loading user files...");
    const nextFiles = await client.adminFiles(user.id);
    setFiles(nextFiles);
    setStatus(`Viewing ${user.email}`);
  }

  async function previewAdminFile(file: FileRecord) {
    if (!canPreview(file)) {
      setStatus("Preview is available for images, videos, and PDFs up to 100 MB.");
      return;
    }
    closePreview();
    setStatus("Preparing admin preview...");
    const download = await client.adminPreviewFile(file);
    const blob = new Blob([bytesToArrayBuffer(download.bytes)], {
      type: download.metadata.mimeType || "application/octet-stream",
    });
    setPreview({ file, url: URL.createObjectURL(blob), mimeType: blob.type });
    setStatus("Preview ready.");
  }

  async function downloadAdminFile(file: FileRecord) {
    setStatus("Preparing admin download...");
    const download = await client.adminDownloadFile(file);
    saveDownloadedFile(download);
    setStatus("Download ready.");
  }

  function saveDownloadedFile(download: DownloadedFile) {
    const blob = new Blob([bytesToArrayBuffer(download.bytes)], {
      type: download.metadata.mimeType || "application/octet-stream",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = download.metadata.name;
    document.body.append(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  return (
    <main className="admin-app">
      {preview ? (
        <div className="modal-backdrop" role="presentation">
          <section aria-modal="true" className="modal preview-modal" role="dialog" aria-labelledby="preview-title">
            <div className="preview-header">
              <h2 id="preview-title">{preview.file.name}</h2>
              <button className="icon-button" type="button" aria-label="Close preview" onClick={closePreview}>
                <X size={18} />
              </button>
            </div>
            <PreviewContent preview={preview} />
          </section>
        </div>
      ) : null}

      <header className="topbar">
        <div className="brand">
          <ShieldCheck size={30} />
          <div>
            <h1>Frorage Admin</h1>
            <p>{status}</p>
          </div>
        </div>
        <a className="text-link" href="/">
          Vault
        </a>
      </header>

      <section className="admin-grid">
        <form className="admin-panel" onSubmit={searchUsers}>
          <h2>User lookup</h2>
          <label>
            Email
            <input value={emailSearch} onChange={(event) => setEmailSearch(event.target.value)} type="email" placeholder="user@example.com" />
          </label>
          <button type="submit">Search</button>
          <div className="admin-list">
            {users.map((user) => (
              <button className="secondary-action" key={user.id} type="button" onClick={() => openUser(user)}>
                {user.email}
              </button>
            ))}
          </div>
        </form>

        <section className="admin-panel">
          <h2>{selectedUser ? selectedUser.email : "Recovered vault"}</h2>
          {selectedUser ? <p className="muted">{selectedUser.storagePrefix}</p> : <p className="muted">Search and choose a user.</p>}
          {currentFolderId ? (
            <button className="secondary-action" type="button" onClick={() => setCurrentFolderId(null)}>
              Back to vault
            </button>
          ) : null}
          <div className="table admin-table">
            {visibleFiles.map((file) => (
              <div className="row" key={file.id}>
                <span className="name-cell">
                  {file.kind === "folder" ? (
                    <button className="folder-link" type="button" onClick={() => setCurrentFolderId(file.id)}>
                      <FolderOpen size={18} />
                      {file.name}
                    </button>
                  ) : (
                    file.name
                  )}
                </span>
                <span>{file.kind}</span>
                <span>{file.ciphertextSize.toLocaleString()}</span>
                <span className="row-actions">
                  {file.kind === "file" ? (
                    <>
                      <button className="row-action" disabled={!canPreview(file)} type="button" onClick={() => previewAdminFile(file)}>
                        <Eye size={18} />
                        Preview
                      </button>
                      <button className="row-action" type="button" onClick={() => downloadAdminFile(file)}>
                        <Download size={18} />
                        Download
                      </button>
                    </>
                  ) : null}
                </span>
              </div>
            ))}
          </div>
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<AdminApp />);
