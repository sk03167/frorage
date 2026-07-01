import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Copy,
  Trash2,
  Download,
  Eye,
  FileUp,
  FolderOpen,
  FolderPlus,
  KeyRound,
  Lock,
  LogIn,
  LogOut,
  Menu,
  MoveRight,
  RefreshCw,
  ShieldCheck,
  X,
} from "lucide-react";
import {
  PrivateCloudClient,
  passwordVerifier,
  type AdminUser,
  type DownloadedFile,
  type FileRecord,
} from "@frorage/sdk";
import "./styles.css";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

type PendingOperation = {
  mode: "move" | "copy";
  ids: string[];
};

type Notice = {
  title: string;
  message: string;
};

type PreviewState = {
  file: FileRecord;
  url: string;
  mimeType: string;
};

const previewLimitBytes = 100 * 1024 * 1024;

if (window.location.pathname === "/admin") {
  createRoot(document.getElementById("root")!).render(<AdminApp />);
} else {
  createRoot(document.getElementById("root")!).render(<App />);
}

function App() {
  const client = useMemo(() => new PrivateCloudClient({ baseUrl: apiBaseUrl }), []);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [signupEmail, setSignupEmail] = useState("");
  const [signupPassword, setSignupPassword] = useState("");
  const [signupOpen, setSignupOpen] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [folderName, setFolderName] = useState("");
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [pendingOperation, setPendingOperation] = useState<PendingOperation | null>(null);
  const [draggedIds, setDraggedIds] = useState<string[]>([]);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [status, setStatus] = useState("Ready");
  const isUnlocked = Boolean(token);
  const visibleFiles = files.filter((file) => (file.parentId ?? null) === currentFolderId);
  const selectedFiles = files.filter((file) => selectedIds.has(file.id));
  const selectedVisibleFiles = visibleFiles.filter((file) => selectedIds.has(file.id));
  const copyableSelection = selectedFiles.filter((file) => file.kind === "file");
  const currentFolderName = currentFolderId ? names[currentFolderId] ?? "Folder" : "Vault";
  const moveHereEnabled = pendingOperation?.mode !== "move" || canMoveTo(currentFolderId, pendingOperation.ids);

  useEffect(() => {
    if (!notice) return;
    function closeOnEnter(event: KeyboardEvent) {
      if (event.key === "Enter") {
        event.preventDefault();
        setNotice(null);
      }
    }
    window.addEventListener("keydown", closeOnEnter);
    return () => window.removeEventListener("keydown", closeOnEnter);
  }, [notice]);

  async function refreshFiles() {
    if (!token) return;
    const nextFiles = await client.listFiles();
    const nextNames: Record<string, string> = {};
    for (const file of nextFiles) {
      nextNames[file.id] = file.name;
    }
    setFiles(nextFiles);
    setNames(nextNames);
    if (currentFolderId && !nextFiles.some((file) => file.id === currentFolderId && file.kind === "folder")) {
      setCurrentFolderId(null);
    }
  }

  async function signup(event: React.FormEvent) {
    event.preventDefault();
    setStatus("Creating encrypted account...");
    try {
      const response = await client.signup(signupEmail, await passwordVerifier(signupEmail, signupPassword));
      setToken(response.token);
      setAccountMenuOpen(false);
      setSignupOpen(false);
      setEmail(signupEmail);
      setPassword("");
      setSignupPassword("");
      setStatus("Account created.");
      setNotice({
        title: "Account created",
        message: "Your Frorage account is ready. If you forget your password, Frorage can reset login access and recover your files.",
      });
      await refreshFiles();
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to create account.";
      setStatus("Ready");
      setNotice({
        title: "Sign up failed",
        message: message === "already exists" ? "An account already exists for this email. Please log in instead." : message,
      });
    }
  }

  async function loginWithPassword(event: React.FormEvent) {
    event.preventDefault();
    setStatus("Checking credentials...");
    try {
      const response = await client.login(email, await passwordVerifier(email, password));
      setToken(response.token);
      setAccountMenuOpen(false);
      setStatus("Vault unlocked.");
      await refreshFiles();
    } catch {
      setStatus("Ready");
      setNotice({
        title: "Invalid credentials",
        message: "We couldn't log you in with that email and password. Please check your credentials, or sign up if you're a new user.",
      });
    }
  }

  function logout() {
    setToken(null);
    setFiles([]);
    setNames({});
    setPassword("");
    setFolderName("");
    setAccountMenuOpen(false);
    setCurrentFolderId(null);
    setSelectedIds(new Set());
    setPendingOperation(null);
    setDraggedIds([]);
    closePreview();
    setStatus("Logged out.");
  }

  function openSignup() {
    setSignupEmail(email);
    setSignupPassword("");
    setSignupOpen(true);
  }

  async function createFolder(event: React.FormEvent) {
    event.preventDefault();
    if (!token) return;
    if (!folderName.trim()) {
      setStatus("Enter a folder name first.");
      return;
    }
    setStatus("Creating encrypted folder...");
    await client.createFolder(currentFolderId, { name: folderName.trim() });
    setFolderName("");
    await refreshFiles();
    setStatus("Folder created.");
  }

  async function uploadSelected(fileList: FileList | null) {
    if (!token || !fileList?.length) return;
    setStatus("Uploading...");
    for (const file of Array.from(fileList)) {
      await client.uploadFile(currentFolderId, file);
    }
    await refreshFiles();
    setStatus("Upload complete.");
  }

  async function downloadSelected(file: FileRecord) {
    if (file.kind !== "file") return;
    setStatus("Preparing download...");
    const download = await client.downloadFile(file);
    saveDownloadedFile(download);
    setStatus("Download ready.");
  }

  async function previewSelected(file: FileRecord) {
    if (file.kind !== "file") return;
    if (!canPreview(file)) {
      setStatus("Preview is available for images, videos, and PDFs up to 100 MB.");
      return;
    }
    closePreview();
    setStatus("Preparing preview...");
    const download = await client.previewFile(file);
    const blob = new Blob([bytesToArrayBuffer(download.bytes)], {
      type: download.metadata.mimeType || "application/octet-stream",
    });
    setPreview({ file, url: URL.createObjectURL(blob), mimeType: blob.type });
    setStatus("Preview ready.");
  }

  function closePreview() {
    setPreview((current) => {
      if (current) URL.revokeObjectURL(current.url);
      return null;
    });
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

  function canMoveTo(targetParentId: string | null, ids: string[]): boolean {
    return ids.every((id) => {
      if (targetParentId === id) return false;
      const item = files.find((file) => file.id === id);
      if (!item || item.kind !== "folder" || !targetParentId) return true;
      let cursor: string | null = targetParentId;
      let guard = 0;
      while (cursor && guard < files.length + 1) {
        if (cursor === id) return false;
        cursor = files.find((file) => file.id === cursor)?.parentId ?? null;
        guard += 1;
      }
      return true;
    });
  }

  function setCurrentFolder(folderId: string | null) {
    setCurrentFolderId(folderId);
    if (!pendingOperation) setSelectedIds(new Set());
  }

  function toggleSelected(fileId: string) {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(fileId)) {
        next.delete(fileId);
      } else {
        next.add(fileId);
      }
      return next;
    });
  }

  function toggleVisibleSelection() {
    const allVisibleSelected = visibleFiles.length > 0 && selectedVisibleFiles.length === visibleFiles.length;
    setSelectedIds((current) => {
      const next = new Set(current);
      for (const file of visibleFiles) {
        if (allVisibleSelected) {
          next.delete(file.id);
        } else {
          next.add(file.id);
        }
      }
      return next;
    });
  }

  function startOperation(mode: PendingOperation["mode"], ids: string[]) {
    const uniqueIds = [...new Set(ids)];
    if (uniqueIds.length === 0) return;
    if (mode === "copy" && uniqueIds.some((id) => files.find((file) => file.id === id)?.kind !== "file")) {
      setStatus("Copy currently supports files only.");
      return;
    }
    setSelectedIds(new Set(uniqueIds));
    setPendingOperation({ mode, ids: uniqueIds });
    setStatus(`${mode === "move" ? "Move" : "Copy"} pending. Open a folder and choose ${mode === "move" ? "Move here" : "Copy here"}.`);
  }

  async function completePendingOperation() {
    if (!pendingOperation) return;
    if (pendingOperation.mode === "move") {
      await moveItems(pendingOperation.ids, currentFolderId);
      return;
    }
    await copyItems(pendingOperation.ids, currentFolderId);
  }

  async function moveItems(ids: string[], targetParentId: string | null) {
    if (!canMoveTo(targetParentId, ids)) {
      setStatus("Choose a different destination.");
      return;
    }
    setStatus("Moving items...");
    for (const id of ids) {
      await client.moveFile(id, targetParentId);
    }
    setSelectedIds(new Set());
    setPendingOperation(null);
    await refreshFiles();
    setStatus(ids.length === 1 ? "Item moved." : `${ids.length} items moved.`);
  }

  async function copyItems(ids: string[], targetParentId: string | null) {
    const items = ids.map((id) => files.find((file) => file.id === id)).filter((file): file is FileRecord => Boolean(file));
    const fileItems = items.filter((file) => file.kind === "file");
    if (fileItems.length !== items.length) {
      setStatus("Copy currently supports files only.");
      return;
    }
    setStatus("Copying files...");
    for (const file of fileItems) {
      await client.copyFile(file, targetParentId);
    }
    setSelectedIds(new Set());
    setPendingOperation(null);
    await refreshFiles();
    setStatus(fileItems.length === 1 ? "File copied." : `${fileItems.length} files copied.`);
  }

  async function deleteSelectedItems() {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    const label = ids.length === 1 ? names[ids[0]] ?? "this item" : `${ids.length} selected items`;
    if (!window.confirm(`Delete ${label}? This removes the record from Frorage.`)) return;
    setStatus("Deleting items...");
    for (const id of ids) {
      await client.deleteFile(id);
    }
    if (currentFolderId && ids.includes(currentFolderId)) {
      setCurrentFolderId(null);
    }
    setSelectedIds(new Set());
    setPendingOperation(null);
    await refreshFiles();
    setStatus(ids.length === 1 ? "Item deleted." : `${ids.length} items deleted.`);
  }

  function folderPath(): FileRecord[] {
    const path: FileRecord[] = [];
    let cursor = currentFolderId;
    let guard = 0;
    while (cursor && guard < files.length + 1) {
      const folder = files.find((file) => file.id === cursor && file.kind === "folder");
      if (!folder) break;
      path.unshift(folder);
      cursor = folder.parentId ?? null;
      guard += 1;
    }
    return path;
  }

  return (
    <main className={`app ${isUnlocked ? "unlocked" : ""}`}>
      {notice ? (
        <div className="modal-backdrop" role="presentation">
          <section aria-modal="true" className="modal" role="dialog" aria-labelledby="notice-title">
            <h2 id="notice-title">{notice.title}</h2>
            <p>{notice.message}</p>
            <button type="button" onClick={() => setNotice(null)}>
              OK
            </button>
          </section>
        </div>
      ) : null}

      {signupOpen ? (
        <div className="modal-backdrop" role="presentation">
          <section aria-modal="true" className="modal" role="dialog" aria-labelledby="signup-title">
            <h2 id="signup-title">Create account</h2>
            <form className="modal-form" onSubmit={signup}>
              <label>
                Email
                <input value={signupEmail} onChange={(event) => setSignupEmail(event.target.value)} type="email" required />
              </label>
              <label>
                Password
                <input value={signupPassword} onChange={(event) => setSignupPassword(event.target.value)} type="password" required />
              </label>
              <div className="modal-actions">
                <button className="secondary-action" type="button" onClick={() => setSignupOpen(false)}>
                  Cancel
                </button>
                <button type="submit">
                  <KeyRound size={18} />
                  Create account
                </button>
              </div>
            </form>
          </section>
        </div>
      ) : null}

      {isUnlocked && accountMenuOpen ? (
        <button className="drawer-backdrop" type="button" aria-label="Close account menu" onClick={() => setAccountMenuOpen(false)} />
      ) : null}

      <aside className={`sidebar ${isUnlocked ? `drawer ${accountMenuOpen ? "is-open" : ""}` : ""}`} aria-hidden={isUnlocked && !accountMenuOpen}>
        <div className="brand">
          <ShieldCheck size={30} />
          <div>
            <h1>Frorage</h1>
            <p>Private encrypted storage.</p>
          </div>
        </div>

        {isUnlocked ? (
          <section className="account">
            <div className="account-menu">
              <p>{email}</p>
              <button type="button" onClick={logout}>
                <LogOut size={18} />
                Log out
              </button>
            </div>
          </section>
        ) : (
          <form className="auth" onSubmit={loginWithPassword}>
            <label>
              Email
              <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
            </label>
            <label>
              Password
              <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" required />
            </label>
            <div className="auth-actions">
              <button type="button" onClick={openSignup}>
                <KeyRound size={18} />
                Sign up
              </button>
              <button type="submit">
                <LogIn size={18} />
                Log in
              </button>
            </div>
          </form>
        )}

      </aside>

      <section className="workspace">
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
          <div className="topbar-title">
            {isUnlocked ? (
              <button
                aria-expanded={accountMenuOpen}
                aria-label="Account menu"
                className="icon-button"
                type="button"
                onClick={() => setAccountMenuOpen((open) => !open)}
              >
                <Menu size={22} />
              </button>
            ) : null}
            <div>
              <h2>Vault</h2>
              <p>{status}</p>
            </div>
          </div>
          <button type="button" onClick={() => refreshFiles()}>
            <RefreshCw size={18} />
            Refresh
          </button>
        </header>

        <div className="toolbar">
          <div className="folder-tools">
            <nav className="breadcrumbs" aria-label="Folder path">
              <button type="button" onClick={() => setCurrentFolder(null)}>
                Vault
              </button>
              {folderPath().map((folder) => (
                <React.Fragment key={folder.id}>
                  <span>/</span>
                  <button type="button" onClick={() => setCurrentFolder(folder.id)}>
                    {names[folder.id] ?? "Folder"}
                  </button>
                </React.Fragment>
              ))}
            </nav>
            <form onSubmit={createFolder}>
              <input value={folderName} onChange={(event) => setFolderName(event.target.value)} placeholder="New encrypted folder" />
              <button disabled={!token || !folderName.trim()} type="submit">
                <FolderPlus size={18} />
                Folder
              </button>
            </form>
          </div>
          <label className="upload">
            <FileUp size={18} />
            Upload
            <input disabled={!token} type="file" multiple onChange={(event) => uploadSelected(event.target.files)} />
          </label>
        </div>

        <div className="selection-bar">
          {pendingOperation ? (
            <>
              <span>
                {pendingOperation.mode === "move" ? "Moving" : "Copying"} {pendingOperation.ids.length}{" "}
                {pendingOperation.ids.length === 1 ? "item" : "items"} to {currentFolderName}
              </span>
              <button disabled={pendingOperation.mode === "move" && !moveHereEnabled} type="button" onClick={completePendingOperation}>
                {pendingOperation.mode === "move" ? <MoveRight size={18} /> : <Copy size={18} />}
                {pendingOperation.mode === "move" ? "Move here" : "Copy here"}
              </button>
              <button
                className="secondary-action"
                type="button"
                onClick={() => {
                  setPendingOperation(null);
                  setSelectedIds(new Set());
                  setStatus("Ready");
                }}
              >
                <X size={18} />
                Cancel
              </button>
            </>
          ) : selectedIds.size > 0 ? (
            <>
              <span>
                {selectedIds.size} {selectedIds.size === 1 ? "item" : "items"} selected
              </span>
              <button type="button" onClick={() => startOperation("move", [...selectedIds])}>
                <MoveRight size={18} />
                Move
              </button>
              <button disabled={copyableSelection.length !== selectedIds.size} type="button" onClick={() => startOperation("copy", [...selectedIds])}>
                <Copy size={18} />
                Copy
              </button>
              <button className="danger-action" type="button" onClick={deleteSelectedItems}>
                <Trash2 size={18} />
                Delete
              </button>
              <button className="secondary-action" type="button" onClick={() => setSelectedIds(new Set())}>
                <X size={18} />
                Clear
              </button>
            </>
          ) : (
            <span>Open folders, select files, or drag files onto folders.</span>
          )}
        </div>

        <div className="table">
          <div className="row head">
            <span>
              <input
                aria-label="Select all visible items"
                checked={visibleFiles.length > 0 && selectedVisibleFiles.length === visibleFiles.length}
                disabled={visibleFiles.length === 0}
                type="checkbox"
                onChange={toggleVisibleSelection}
              />
            </span>
            <span>Name</span>
            <span>Type</span>
            <span>Encrypted bytes</span>
            <span>Actions</span>
          </div>
          {visibleFiles.length === 0 ? (
            <div className="empty">
              <Lock size={28} />
              <p>{token ? "No files yet." : "Log in or sign up to unlock your vault."}</p>
            </div>
          ) : (
            visibleFiles.map((file) => (
              <div
                className={`row ${file.kind === "folder" ? "folder-row" : ""}`}
                draggable={isUnlocked}
                key={file.id}
                onDragStart={(event) => {
                  const ids = selectedIds.has(file.id) ? [...selectedIds] : [file.id];
                  setDraggedIds(ids);
                  event.dataTransfer.effectAllowed = "move";
                  event.dataTransfer.setData("text/plain", ids.join(","));
                }}
                onDragOver={(event) => {
                  if (file.kind === "folder" && canMoveTo(file.id, draggedIds)) {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = "move";
                  }
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  if (file.kind === "folder" && draggedIds.length > 0) {
                    void moveItems(draggedIds, file.id);
                    setDraggedIds([]);
                  }
                }}
              >
                <span>
                  <input
                    aria-label={`Select ${names[file.id] ?? "item"}`}
                    checked={selectedIds.has(file.id)}
                    type="checkbox"
                    onChange={() => toggleSelected(file.id)}
                  />
                </span>
                <span className="name-cell">
                  {file.kind === "folder" ? (
                    <button className="folder-link" type="button" onClick={() => setCurrentFolder(file.id)}>
                      <FolderOpen size={18} />
                      {names[file.id] ?? "Folder"}
                    </button>
                  ) : (
                    names[file.id] ?? "Encrypted item"
                  )}
                </span>
                <span>{file.kind}</span>
                <span>{file.ciphertextSize.toLocaleString()}</span>
                <span className="row-actions">
                  {file.kind === "file" ? (
                    <>
                      <button className="row-action" disabled={!canPreview(file)} type="button" onClick={() => previewSelected(file)}>
                        <Eye size={18} />
                        Preview
                      </button>
                      <button className="row-action" type="button" onClick={() => downloadSelected(file)}>
                        <Download size={18} />
                        Download
                      </button>
                    </>
                  ) : (
                    <span className="muted">Drop files here</span>
                  )}
                </span>
              </div>
            ))
          )}
        </div>
      </section>
    </main>
  );
}

function AdminApp() {
  const client = useMemo(() => new PrivateCloudClient({ baseUrl: apiBaseUrl }), []);
  const [adminToken, setAdminToken] = useState("");
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

  async function searchUsers(event: React.FormEvent) {
    event.preventDefault();
    client.setAdminToken(adminToken);
    setStatus("Searching users...");
    const nextUsers = await client.adminUsers(emailSearch);
    setUsers(nextUsers);
    setSelectedUser(null);
    setFiles([]);
    setCurrentFolderId(null);
    setStatus(nextUsers.length === 0 ? "No users found." : `${nextUsers.length} user found.`);
  }

  async function openUser(user: AdminUser) {
    client.setAdminToken(adminToken);
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
    client.setAdminToken(adminToken);
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
    client.setAdminToken(adminToken);
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
            Admin token
            <input value={adminToken} onChange={(event) => setAdminToken(event.target.value)} type="password" required />
          </label>
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

function PreviewContent({ preview }: { preview: PreviewState }) {
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

function canPreview(file: FileRecord): boolean {
  if (file.kind !== "file" || file.ciphertextSize > previewLimitBytes) return false;
  const mimeType = file.mimeType ?? "";
  return mimeType.startsWith("image/") || mimeType.startsWith("video/") || mimeType === "application/pdf";
}

function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}
