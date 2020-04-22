import { App } from "@aws-cdk/core";
import { NATTestResourcesStack } from "./NATTestResourcesStack";

export class NATTestResourcesApp extends App {
  public constructor({ stackId }: { stackId: string }) {
    super();
    new NATTestResourcesStack(this, stackId);
  }
}
