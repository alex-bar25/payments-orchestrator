import assert from "node:assert/strict";
import test from "node:test";
import { actionsFor } from "./payment-ui.ts";

test("actions follow the payment status machine", () => {
  assert.deepEqual(actionsFor("requires_payment"), ["authorize"]);
  assert.deepEqual(actionsFor("processing"), []);
  assert.deepEqual(actionsFor("authorized"), ["capture", "cancel"]);
  assert.deepEqual(actionsFor("captured"), ["refund"]);
  assert.deepEqual(actionsFor("refunded"), []);
  assert.deepEqual(actionsFor("failed"), []);
  assert.deepEqual(actionsFor("cancelled"), []);
});
