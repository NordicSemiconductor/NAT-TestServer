import { NATTestResourcesApp } from "./NATTestResourcesApp";

const STACK_ID = process.env.STACK_ID || "nat-test-resources";

new NATTestResourcesApp({
  stackId: STACK_ID,
}).synth();
