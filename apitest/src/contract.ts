// Validates response bodies against the component schemas in api/openapi.yaml.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import Ajv2020 from "ajv/dist/2020.js";
import { parse } from "yaml";

const spec = parse(readFileSync(join(import.meta.dirname, "../../api/openapi.yaml"), "utf8"));

// OpenAPI 3.1 schemas are JSON Schema 2020-12; register them under one id so
// that "#/components/schemas/X" references resolve.
const ajv = new (Ajv2020 as unknown as typeof Ajv2020.default)({ strict: false, allErrors: true });
ajv.addSchema({ $id: "openapi", components: spec.components }, "openapi");

/** Throws unless body matches the JSON schema documented for operationId at status. */
export function assertOperation(operationId: string, status: number, body: unknown): void {
  for (const [path, item] of Object.entries<any>(spec.paths)) {
    for (const op of Object.values<any>(item)) {
      if (op?.operationId !== operationId) continue;
      let res = op.responses[String(status)];
      if (!res) throw new Error(`${operationId} (${path}) does not document status ${status}`);
      if (res.$ref) res = spec.components.responses[res.$ref.split("/").pop()];
      const schema = res.content?.["application/json"]?.schema;
      if (!schema) throw new Error(`${operationId} ${status} has no JSON schema in the spec`);
      const validate = ajv.compile({ $schema: "https://json-schema.org/draft/2020-12/schema", ...rewriteRefs(schema) });
      if (!validate(body)) {
        throw new Error(`${operationId} ${status} does not match the spec:\n${ajv.errorsText(validate.errors, { separator: "\n" })}`);
      }
      return;
    }
  }
  throw new Error(`unknown operation ${operationId}`);
}

function rewriteRefs(node: any): any {
  if (Array.isArray(node)) return node.map(rewriteRefs);
  if (node && typeof node === "object") {
    const out: any = {};
    for (const [k, v] of Object.entries(node)) {
      out[k] = k === "$ref" && typeof v === "string" && v.startsWith("#/") ? "openapi" + v : rewriteRefs(v);
    }
    return out;
  }
  return node;
}
