import { describe, expect, it } from "vitest";
import { passwordVerifier } from "./crypto";

describe("crypto", () => {
  it("derives stable password verifiers for normalized email", async () => {
    const verifier = await passwordVerifier(" USER@example.com ", "correct horse battery staple");
    await expect(passwordVerifier("user@example.com", "correct horse battery staple")).resolves.toEqual(verifier);
    await expect(passwordVerifier("user@example.com", "wrong password")).resolves.not.toEqual(verifier);
  });
});
