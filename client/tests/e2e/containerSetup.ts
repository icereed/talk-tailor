import { GenericContainer, StartedTestContainer, Wait } from "testcontainers";

let container: StartedTestContainer;

export async function startAppContainer() {
  const openaiApiKey = process.env.OPENAI_API_KEY;
  if (!openaiApiKey) throw new Error("OPENAI_API_KEY must be set in env");

  container = await new GenericContainer("talk-tailor:latest")
    .withEnvironment({ OPENAI_API_KEY: openaiApiKey })
    .withExposedPorts(8080)
    .withWaitStrategy(Wait.forListeningPorts())
    .start();

  const port = container.getMappedPort(8080);
  const host = container.getHost();
  return { host, port, container };
}

export async function stopAppContainer() {
  if (container) await container.stop();
}
