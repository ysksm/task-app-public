// Logging in (01 §4.12): the login page, passwords set in Settings, logout.
// With --require-login the browser is treated like another device: no
// automatic owner login.
import { expect, test } from "../fixtures.js";

test.describe("from another device (--require-login)", () => {
  test.use({ serveArgs: ["--require-login"] });

  test("sends to the login page and back to the requested page after logging in", async ({ page, demo }) => {
    await demo.api("owner").put("/api/v1/users/alice/password", { password: "alice-pass-1" });

    await page.goto("/inbox/qa");
    await expect(page).toHaveURL(/\/login\?next=%2Finbox%2Fqa$/);
    await expect(page.getByRole("button", { name: "このPCのオーナーとして入る" })).toHaveCount(0);

    await page.getByLabel("ユーザー名").fill("alice");
    await page.getByLabel("パスワード").fill("wrong-password");
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page.getByRole("alert")).toHaveText("ユーザー名かパスワードが違います");

    await page.getByLabel("パスワード").fill("alice-pass-1");
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/inbox\/qa$/);
    await expect(page.getByRole("navigation").getByText("Alice")).toBeVisible();
  });

  test("hides the owner's settings from other people", async ({ page, demo }) => {
    await demo.api("owner").put("/api/v1/users/alice/password", { password: "alice-pass-1" });
    await page.goto("/login");
    await page.getByLabel("ユーザー名").fill("alice");
    await page.getByLabel("パスワード").fill("alice-pass-1");
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/my$/);

    await page.goto("/settings");
    await expect(page.getByTestId("account").getByText("@alice")).toBeVisible();
    await expect(page.getByTestId("auto-backup")).toHaveCount(0);
    await expect(page.getByRole("link", { name: "JSON をエクスポート" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "エージェントを追加" })).toHaveCount(0);
  });

  test("logs out", async ({ page, demo }) => {
    await demo.api("owner").put("/api/v1/users/owner/password", { password: "owner-pass-1" });
    await page.goto("/login");
    await page.getByLabel("ユーザー名").fill("owner");
    await page.getByLabel("パスワード").fill("owner-pass-1");
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/my$/);

    await page.getByRole("button", { name: "ログアウト" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await page.goto("/my");
    await expect(page).toHaveURL(/\/login\?next=%2Fmy$/);
  });
  test("keeps me logged in across a restart (a deploy) when asked", async ({ page, context, demo }) => {
    await demo.api("owner").put("/api/v1/users/alice/password", { password: "alice-pass-1" });
    await page.goto("/login");
    await page.getByLabel("ユーザー名").fill("alice");
    await page.getByLabel("パスワード").fill("alice-pass-1");
    await page.getByLabel("ログインしたままにする（30 日）").check();
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/my$/);
    const cookie = (await context.cookies()).find((c) => c.name === "taskapp_session")!;
    expect(cookie.expires).toBeGreaterThan(Date.now() / 1000 + 29 * 86400); // persistent, not a browser-session cookie
    // The box stays checked next time on this browser.
    await page.goto("/login");
    await expect(page.getByLabel("ログインしたままにする（30 日）")).toBeChecked();

    await demo.server.restart(); // new port, same host: the cookie still applies
    await page.goto(demo.server.baseUrl + "/my");
    await expect(page).toHaveURL(/\/my$/);
    await expect(page.getByRole("navigation").getByText("Alice")).toBeVisible();

  });
});

test.describe("on this machine", () => {
  test("sets the owner's password in Settings and logs in with it after logging out", async ({ page, demo }) => {
    await page.goto("/settings");
    const account = page.getByTestId("account");
    await expect(account.getByText("パスワード未設定")).toBeVisible();
    await account.getByPlaceholder("新しいパスワード").fill("owner-pass-1");
    await account.getByPlaceholder("もう一度").fill("owner-pass-1");
    await account.getByRole("button", { name: "パスワードを設定" }).click();
    await expect(account.getByText("設定しました")).toBeVisible();
    expect((await demo.api("owner").get("/api/v1/me")).body.has_password).toBe(true);

    // After an explicit logout this machine does not log straight back in.
    await page.getByRole("button", { name: "ログアウト" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await page.goto("/my");
    await expect(page).toHaveURL(/\/login\?next=%2Fmy$/);

    await page.getByLabel("ユーザー名").fill("owner");
    await page.getByLabel("パスワード").fill("owner-pass-1");
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/my$/);
  });

  test("offers automatic login again on the login page", async ({ page }) => {
    await page.goto("/my");
    await page.getByRole("button", { name: "ログアウト" }).click();
    await page.getByRole("button", { name: "このPCのオーナーとして入る" }).click();
    await expect(page).toHaveURL(/\/my$/);
    await expect(page.getByRole("navigation").getByText("Owner")).toBeVisible();
    // and stays logged in automatically from now on
    await page.reload();
    await expect(page).toHaveURL(/\/my$/);
  });

  test("lets the owner set another person's password", async ({ page, demo }) => {
    await page.goto("/settings");
    await page.getByTestId("member-alice").getByRole("button", { name: "パスワードを設定" }).click();
    await expect(page.getByText("@alice の新しいパスワード")).toBeVisible();
    const form = page.locator("form", { hasText: "@alice の新しいパスワード" });
    await form.getByPlaceholder("新しいパスワード").fill("alice-pass-1");
    await form.getByPlaceholder("もう一度").fill("alice-pass-1");
    await form.getByRole("button", { name: "パスワードを設定" }).click();
    await expect(page.getByText("@alice の新しいパスワード")).toHaveCount(0);
    const users = (await demo.api("owner").get("/api/v1/users")).body.users;
    expect(users.find((u: { username: string }) => u.username === "alice").has_password).toBe(true);
  });

  test("shows how to connect from other devices", async ({ page }) => {
    await page.goto("/settings");
    const remote = page.getByTestId("remote-access");
    await expect(remote.getByText("今はこの PC からだけ接続できます。", { exact: false })).toBeVisible();
    await expect(remote.getByText(/serve .*--host 0\.0\.0\.0$/)).toBeVisible();
  });
});
