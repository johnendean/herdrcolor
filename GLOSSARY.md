# Glossary

**Agent**: a pane that Herdr reports as running an AI coding agent. It is coloured by its project.

**Project**: the last component of the directory Herdr reports for an agent, meaning its name rather than its full path. A project is what a colour identifies. On Windows, that directory is where the agent was launched.

**Space**: a Herdr workspace as it appears in the spaces panel. A space takes the colour of the agents inside it.

**Palette**: the fixed, ordered set of six colours a project can be given.

**Slot**: one of six sidebar tokens, `$c1` to `$c6`, each permanently tied to one palette colour. A row is coloured by which slot holds its name.

**Managed block**: the part of the user's Herdr config that herdrcolor writes and owns, set off by begin and end markers. herdrcolor changes text only inside its managed block.

**Hand-written section**: a sidebar section the user defined outside the managed block. herdrcolor never edits one, and reports it as a conflict instead.

**Install**: the one command that takes a machine from nothing to a coloured sidebar, `herdr plugin install johnendean/herdrcolor`.
