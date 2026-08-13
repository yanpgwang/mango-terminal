# Product direction

## Core journey

```text
login -> Session inbox -> attach -> observe -> approve or interrupt -> detach
```

Detaching closes only the local connection. The Session continues remotely and
can be attached again from another machine.

## Attached Session

The attached view has three responsibilities:

1. Present the selected Thread as a readable timeline.
2. Keep the activity and status of every other Thread visible.
3. Surface actions that require a human without turning the terminal into a
   control-plane administration console.

Timeline items are semantic rather than raw event rows:

- user and agent messages;
- thinking lifecycle;
- delegation task and child report;
- tool call and result;
- permission request and resolution;
- Thread lifecycle and errors.

## Delivery sequence

### Attach foundation

- Session inbox and direct attach;
- reconnect from durable ledgers;
- concurrent primary/child SSE streams;
- delegation, report, thinking, and message projection;
- message and interrupt controls.

### Interactive execution

- [x] expandable paired tool calls and results;
- [x] permission overlay with child ownership for built-in confirmations;
- [x] semantic delegation and report components;
- [x] timeline, composer, and Thread-roster focus modes;
- [x] command palette for Session controls;
- custom and self-hosted tool results;
- robust delta reconciliation and reconnect cursors;
- background activity summaries and terminal notifications.

### Managed resources

- Session creation;
- Files and attachments;
- Memory Store, Skill, and Vault visibility;
- outcome evaluation;
- usage and cost inspection.

### Product shell

- endpoint profiles and login;
- secure credential storage;
- command palette, mouse navigation, themes, and accessibility;
- packaged releases and update flow.

Agent, Environment, Skill, Memory, Vault, and Deployment administration stays
outside the terminal unless a concrete user workflow requires it.
