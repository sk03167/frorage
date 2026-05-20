import React, { useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { FileUp, FolderPlus, KeyRound, Lock, LogIn, RefreshCw, ShieldCheck } from "lucide-react";
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

function App() {
  const client = useMemo(() => new PrivateCloudClient({ baseUrl: apiBaseUrl }), []);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [masterKey, setMasterKey] = useState<CryptoKey | null>(null);
  const [recoveryKit, setRecoveryKit] = useState<RecoveryKit | null>(null);
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [folderName, setFolderName] = useState("");
  const [status, setStatus] = useState("Ready");

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
  }

  async function signup(event: React.FormEvent) {
    event.preventDefault();
    setStatus("Creating encrypted account...");
    const account = await createAccountCrypto(email, password);
    const response = await client.signup(email, account.passwordVerifier, account.keyBundle);
    setToken(response.token);
    setMasterKey(account.masterKey);
    setRecoveryKit(account.recoveryKit);
    setStatus("Account created. Save your recovery phrase and file before uploading important data.");
    await refreshFiles(account.masterKey);
  }

  async function loginWithPassword(event: React.FormEvent) {
    event.preventDefault();
    setStatus("Checking credentials...");
    const response = await client.login(email, await passwordVerifier(email, password));
    const key = await unlockWithPassword(password, response.keyBundle);
    setToken(response.token);
    setMasterKey(key);
    setStatus("Vault unlocked.");
    await refreshFiles(key);
  }

  async function createFolder(event: React.FormEvent) {
    event.preventDefault();
    if (!masterKey || !folderName.trim()) return;
    setStatus("Creating encrypted folder...");
    await client.createFolder(masterKey, null, { name: folderName.trim() });
    setFolderName("");
    await refreshFiles();
    setStatus("Folder created.");
  }

  async function uploadSelected(fileList: FileList | null) {
    if (!masterKey || !fileList?.length) return;
    setStatus("Encrypting and uploading...");
    for (const file of Array.from(fileList)) {
      await client.uploadFile(masterKey, null, file);
    }
    await refreshFiles();
    setStatus("Upload complete.");
  }

  return (
    <main className="app">
      <aside className="sidebar">
        <div className="brand">
          <ShieldCheck size={30} />
          <div>
            <h1>Private Cloud</h1>
            <p>Encrypted storage, provider-priced.</p>
          </div>
        </div>

        <form className="auth" onSubmit={token ? loginWithPassword : signup}>
          <label>
            Email
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
          </label>
          <label>
            Password
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" required />
          </label>
          <div className="auth-actions">
            <button type="submit">
              <KeyRound size={18} />
              Sign up
            </button>
            <button type="button" onClick={loginWithPassword}>
              <LogIn size={18} />
              Log in
            </button>
          </div>
        </form>

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
          <div>
            <h2>Vault</h2>
            <p>{status}</p>
          </div>
          <button type="button" onClick={() => refreshFiles()}>
            <RefreshCw size={18} />
            Refresh
          </button>
        </header>

        <div className="toolbar">
          <form onSubmit={createFolder}>
            <input value={folderName} onChange={(event) => setFolderName(event.target.value)} placeholder="New encrypted folder" />
            <button disabled={!masterKey} type="submit">
              <FolderPlus size={18} />
              Folder
            </button>
          </form>
          <label className="upload">
            <FileUp size={18} />
            Upload
            <input disabled={!masterKey} type="file" multiple onChange={(event) => uploadSelected(event.target.files)} />
          </label>
        </div>

        <div className="table">
          <div className="row head">
            <span>Name</span>
            <span>Type</span>
            <span>Encrypted bytes</span>
          </div>
          {files.length === 0 ? (
            <div className="empty">
              <Lock size={28} />
              <p>{masterKey ? "No files yet." : "Log in or sign up to unlock your encrypted vault."}</p>
            </div>
          ) : (
            files.map((file) => (
              <div className="row" key={file.id}>
                <span>{names[file.id] ?? "Encrypted item"}</span>
                <span>{file.kind}</span>
                <span>{file.ciphertextSize.toLocaleString()}</span>
              </div>
            ))
          )}
        </div>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
