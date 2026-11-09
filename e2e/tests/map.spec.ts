// Mind map (MAP-01..04): render, collapse, drag to reparent, PNG export.
import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures.js";

const node = (page: Page, key: string) => page.locator(`.react-flow__node[data-id="${key}"]`);

/** Drags one node onto another with real mouse moves (React Flow ignores synthetic drag events). */
async function dragOnto(page: Page, from: string, to: string) {
  const a = (await node(page, from).boundingBox())!;
  const b = (await node(page, to).boundingBox())!;
  await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 12 });
  await page.mouse.up();
}

test.beforeEach(async ({ page }) => {
  await page.goto("/p/APP/issues?view=map");
  await expect(node(page, "APP-1")).toBeVisible();
});

test("shows the hierarchy and collapses it by depth", async ({ page }) => {
  await expect(node(page, "APP-6")).toBeVisible();
  await page.getByRole("button", { name: "1", exact: true }).click();
  await expect(node(page, "APP-6")).toHaveCount(0);
  await expect(node(page, "APP-1")).toBeVisible();
  await page.getByRole("button", { name: "すべて" }).click();
  await expect(node(page, "APP-6")).toBeVisible();
});

test("reparents a node by dragging it onto another", async ({ page, demo }) => {
  await dragOnto(page, "APP-21", "APP-19");
  await expect(page.getByText("APP-21 を APP-19 に移動しました")).toBeVisible();
  expect((await demo.api("owner").get("/api/v1/issues/APP-21")).body.parent_key).toBe("APP-19");
});

test("refuses a move that breaks the hierarchy and explains why", async ({ page, demo }) => {
  await dragOnto(page, "APP-17", "APP-14");
  await expect(page.getByText(/^移動できません/)).toBeVisible();
  expect((await demo.api("owner").get("/api/v1/issues/APP-17")).body.parent_key).toBe("APP-12");
});

test("saves the map as PNG", async ({ page }) => {
  const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "PNG" }).click()]);
  expect(download.suggestedFilename()).toMatch(/\.png$/);
});
