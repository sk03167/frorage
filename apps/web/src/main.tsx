import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Copy,
  Trash2,
  Download,
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
  createAccountCrypto,
  decryptMetadata,
  passwordVerifier,
  unlockWithPassword,
  type FileRecord,
  type RecoveryKit,
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

function App() {
  const client = useMemo(() => new PrivateCloudClient({ baseUrl: apiBaseUrl }), []);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [signupEmail, setSignupEmail] = useState("");
  const [signupPassword, setSignupPassword] = useState("");
  const [signupOpen, setSignupOpen] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const [masterKey, setMasterKey] = useState<CryptoKey | null>(null);
  const [recoveryKit, setRecoveryKit] = useState<RecoveryKit | null>(null);
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [folderName, setFolderName] = useState("");
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [pendingOperation, setPendingOperation] = useState<PendingOperation | null>(null);
  const [draggedIds, setDraggedIds] = useState<string[]>([]);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [status, setStatus] = useState("Ready");
  const isUnlocked = Boolean(token && masterKey);
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

  async function refreshFiles(key = masterKey) {
    if (!key) return;
    const nextFiles = await client.listFiles();
    const nextNames: Record<string, string> = {};
    for (const file of nextFiles) {
      const metadata = await decryptMetadata(key, file.encryptedMetadata);
      nextNames[file.id] = metadata.name;
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
      const account = await createAccountCrypto(signupEmail, signupPassword);
      const response = await client.signup(signupEmail, account.passwordVerifier, account.keyBundle);
      setToken(response.token);
      setMasterKey(account.masterKey);
      setRecoveryKit(account.recoveryKit);
      setAccountMenuOpen(false);
      setSignupOpen(false);
      setEmail(signupEmail);
      setPassword("");
      setSignupPassword("");
      setStatus("Account created. Save your recovery phrase and file before uploading important data.");
      setNotice({
        title: "Account created",
        message: "Your encrypted Frorage account is ready. Save your recovery phrase and recovery file before uploading important data.",
      });
      await refreshFiles(account.masterKey);
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
      const key = await unlockWithPassword(password, response.keyBundle);
      setToken(response.token);
      setMasterKey(key);
      setAccountMenuOpen(false);
      setStatus("Vault unlocked.");
      await refreshFiles(key);
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
    setMasterKey(null);
    setRecoveryKit(null);
    setFiles([]);
    setNames({});
    setPassword("");
    setFolderName("");
    setAccountMenuOpen(false);
    setCurrentFolderId(null);
    setSelectedIds(new Set());
    setPendingOperation(null);
    setDraggedIds([]);
    setStatus("Logged out.");
  }

  function openSignup() {
    setSignupEmail(email);
    setSignupPassword("");
    setSignupOpen(true);
  }

  async function createFolder(event: React.FormEvent) {
    event.preventDefault();
    if (!masterKey) return;
    if (!folderName.trim()) {
      setStatus("Enter a folder name first.");
      return;
    }
    setStatus("Creating encrypted folder...");
    await client.createFolder(masterKey, currentFolderId, { name: folderName.trim() });
    setFolderName("");
    await refreshFiles();
    setStatus("Folder created.");
  }

  async function uploadSelected(fileList: FileList | null) {
    if (!masterKey || !fileList?.length) return;
    setStatus("Encrypting and uploading...");
    for (const file of Array.from(fileList)) {
      await client.uploadFile(masterKey, currentFolderId, file);
    }
    await refreshFiles();
    setStatus("Upload complete.");
  }

  async function downloadSelected(file: FileRecord) {
    if (!masterKey || file.kind !== "file") return;
    setStatus("Decrypting download...");
    const download = await client.downloadFile(masterKey, file);
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
    setStatus("Download ready.");
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
    if (!masterKey) return;
    const items = ids.map((id) => files.find((file) => file.id === id)).filter((file): file is FileRecord => Boolean(file));
    const fileItems = items.filter((file) => file.kind === "file");
    if (fileItems.length !== items.length) {
      setStatus("Copy currently supports files only.");
      return;
    }
    setStatus("Copying files...");
    for (const file of fileItems) {
      await client.copyFile(masterKey, file, targetParentId);
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

        {recoveryKit ? (
          <section className="recovery">
            <h2>Recovery kit</h2>
            <p>{recoveryKit.phrase}</p>
            <a
              download="frorage-recovery.json"
              href={`data:application/json,${encodeURIComponent(JSON.stringify(recoveryKit.file, null, 2))}`}
            >
              Download recovery file
            </a>
          </section>
        ) : null}
      </aside>

      <section className="workspace">
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
              <button disabled={!masterKey || !folderName.trim()} type="submit">
                <FolderPlus size={18} />
                Folder
              </button>
            </form>
          </div>
          <label className="upload">
            <FileUp size={18} />
            Upload
            <input disabled={!masterKey} type="file" multiple onChange={(event) => uploadSelected(event.target.files)} />
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
              <p>{masterKey ? "No files yet." : "Log in or sign up to unlock your encrypted vault."}</p>
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
                    <button className="row-action" type="button" onClick={() => downloadSelected(file)}>
                      <Download size={18} />
                      Download
                    </button>
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

createRoot(document.getElementById("root")!).render(<App />);

function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}
