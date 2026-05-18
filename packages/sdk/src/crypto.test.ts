import { describe, expect, it } from "vitest";
import {
  createAccountCrypto,
  decryptBytes,
  decryptMetadata,
  encryptBytes,
  encryptMetadata,
  unlockWithPassword,
  unlockWithRecoveryFile,
  unlockWithRecoveryPhrase,
} from "./crypto";

describe("crypto", () => {
  it("wraps the same account master key for password, phrase, and recovery file", async () => {
    const account = await createAccountCrypto("user@example.com", "correct horse battery staple");
    const byPassword = await unlockWithPassword("correct horse battery staple", account.keyBundle);
    const byPhrase = await unlockWithRecoveryPhrase(account.recoveryKit.phrase, account.keyBundle);
    const byFile = await unlockWithRecoveryFile(account.recoveryKit.file, account.keyBundle);

    const encrypted = await encryptMetadata(account.masterKey, { name: "taxes.pdf" });
    await expect(decryptMetadata(byPassword, encrypted)).resolves.toEqual({ name: "taxes.pdf" });
    await expect(decryptMetadata(byPhrase, encrypted)).resolves.toEqual({ name: "taxes.pdf" });
    await expect(decryptMetadata(byFile, encrypted)).resolves.toEqual({ name: "taxes.pdf" });
  });

  it("encrypts and decrypts file bytes", async () => {
    const account = await createAccountCrypto("user@example.com", "password");
    const payload = new TextEncoder().encode("secret file");
    const encrypted = await encryptBytes(account.masterKey, payload);
    expect(encrypted).not.toEqual(payload);
    await expect(decryptBytes(account.masterKey, encrypted)).resolves.toEqual(payload);
  });
});
