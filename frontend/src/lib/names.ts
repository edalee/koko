const adjectives = [
  "morning",
  "swift",
  "quiet",
  "bright",
  "cosmic",
  "amber",
  "crystal",
  "golden",
  "lunar",
  "velvet",
  "misty",
  "silver",
  "wild",
  "gentle",
  "bold",
  "crimson",
  "azure",
  "sage",
  "ember",
  "frost",
];

const nouns = [
  "fox",
  "river",
  "spark",
  "cloud",
  "pine",
  "coral",
  "raven",
  "storm",
  "grove",
  "pearl",
  "falcon",
  "meadow",
  "harbor",
  "ridge",
  "ember",
  "reef",
  "hawk",
  "dune",
  "moss",
  "crest",
];

export function randomName(): string {
  const adj = adjectives[Math.floor(Math.random() * adjectives.length)];
  const noun = nouns[Math.floor(Math.random() * nouns.length)];
  return `${adj}-${noun}`;
}
