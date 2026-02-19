# ClusterVersion Progress Insight Controller — Specification

## Overview

This controller tracks control plane update progress in standalone OpenShift clusters. It watches the `clusterversions.config.openshift.io` resource (almost always named `version`) and maintains a corresponding `ClusterVersionProgressInsight` CRD that summarizes the update state in a form convenient for tooling and UIs.

As a secondary output, the controller also manages `UpdateHealthInsight` CRDs that describe discrete health observations about the update. At present only one category of health insight is produced (see §9).

**Primary input:** `clusterversions.config.openshift.io` (read-only)
**Secondary input:** `clusteroperators.config.openshift.io` (read-only, for completion tracking)
**Primary output:** `ClusterVersionProgressInsight` (one per ClusterVersion, same name)
**Secondary output:** `UpdateHealthInsight` (zero or more, owned by the ClusterVersionProgressInsight)

---

## 1. Trigger Conditions

The reconciler runs whenever any of the following events occur:

- **ClusterVersionProgressInsight changes** — the insight itself is the primary reconciliation target.
- **ClusterVersion changes** — any change to the watched ClusterVersion triggers reconciliation under the same name.
- **ClusterOperator "operator" version changes** — reconciliation is triggered when a ClusterOperator's `status.versions[name=="operator"].version` field changes (update events only). Other ClusterOperator changes are filtered out to avoid unnecessary churn.

When triggered by a ClusterOperator event, the reconciler is always enqueued under the name `version` (the conventional name for the singleton ClusterVersion resource).

---

## 2. Lifecycle: ClusterVersionProgressInsight

The controller ensures a one-to-one correspondence between ClusterVersion resources and ClusterVersionProgressInsight resources:

- **Create:** When a ClusterVersion exists but no corresponding ClusterVersionProgressInsight does, the controller creates the insight object first (to obtain a resourceVersion), then updates its status in the same reconcile loop.
- **Delete:** When the ClusterVersion no longer exists but a ClusterVersionProgressInsight does, the controller deletes the insight.
- **Nothing to do:** When neither exists, the controller returns early without action.
- **Race condition handling:** If creating the insight fails with `AlreadyExists` (concurrent reconciliation created it), the controller requeues after 1 second rather than erroring. Similarly, if updating the status fails with a conflict error, the controller requeues after 1 second.

---

## 3. Assessment

The `assessment` field is a human-oriented summary of the overall update state. It is derived directly from the `Updating` condition status:

| Updating condition status | Assessment    |
|---------------------------|---------------|
| `True`                    | `Progressing` |
| `False`                   | `Completed`   |
| `Unknown`                 | `Unknown`     |

The `Degraded` assessment value is defined in the API but is not currently set by the controller (no degraded detection is implemented yet).

---

## 4. Conditions

### Updating Condition

The `Updating` condition reports whether the control plane is currently undergoing an update. It is determined by cross-referencing the ClusterVersion `Progressing` status condition with the most recent update history entry (`history[0]`):

**Updating=True (Reason: `Progressing`)** when:
- `ClusterVersion.Status.Conditions[Progressing].Status == True`, AND
- `history[0].State == PartialUpdate`, AND
- `history[0].CompletionTime` is absent

**Updating=False (Reason: `NotProgressing`)** when:
- `ClusterVersion.Status.Conditions[Progressing].Status == False`, AND
- `history[0].State == CompletedUpdate`, AND
- `history[0].CompletionTime` is present

**Updating=Unknown (Reason: `CannotDetermineUpdating`)** in all other cases:
- The `Progressing` condition is absent from ClusterVersion
- The history is empty
- `Progressing=True` but `history[0]` is not `PartialUpdate`
- `Progressing=True` but `history[0]` has a CompletionTime (internally inconsistent)
- `Progressing=False` but `history[0]` is not `CompletedUpdate`
- `Progressing=False` but `history[0]` has no CompletionTime

The condition message always includes the full ClusterVersion Progressing condition status, reason, and message in the format:
```
ClusterVersion has Progressing={Status}(Reason={Reason}) | Message='{Message}'
```

---

## 5. Completion

`completionPercent` represents how far the control plane update has progressed, expressed as an integer percentage (0–100).

**When the assessment is Completed:** completion is forced to `100` regardless of ClusterOperator states. This handles the case where ClusterVersion reports completion but individual operators have not yet caught up (an inconsistent but valid transient state).

**When the assessment is Progressing or Unknown:** completion is computed from ClusterOperator versions:
1. For each ClusterOperator, look for a version entry with `name == "operator"`.
2. Count operators whose `"operator"` version equals `ClusterVersion.Status.Desired.Version`.
3. `completion = floor((updatedOperators / totalOperators) * 100)`

Operators without an `"operator"` version entry do not count as updated (they are treated as pending). If there are no ClusterOperators at all, completion is 0.

---

## 6. Version Information

Versions are extracted from `ClusterVersion.Status.History`:

- **Target**: `history[0].Version` (the current/most recent update target)
- **Previous**: `history[1].Version` if at least two history entries exist; absent otherwise

**Metadata flags:**
- Target receives the `Installation` metadata flag when there is no Previous version (single history entry). This marks the initial cluster installation.
- Previous receives the `Partial` metadata flag when `history[1].State == PartialUpdate`. This indicates the previous update was never fully completed before this update started.

---

## 7. Timing Fields

All timing information is extracted from `ClusterVersion.Status.History[0]` (the current or most recent update entry), not from the ClusterVersion conditions (which can have slightly different timestamps).

| Field                 | Source                               | Presence                                |
|-----------------------|--------------------------------------|-----------------------------------------|
| `startedAt`           | `history[0].StartedTime`             | Always set (when Progressing or Completed) |
| `completedAt`         | `history[0].CompletionTime`          | Only when `Assessment == Completed`     |
| `lastObservedProgress`| Updated when `completionPercent` changes | Always set; preserved when no change |
| `estimatedCompletedAt`| Computed from elapsed time and progress | Only when `Assessment != Completed`   |

### `lastObservedProgress`

This timestamp records when completion progress was last observed to change. It is updated to "now" whenever:
- The `completionPercent` value changes from the previous reconciliation, OR
- It has never been set (zero value)

It is preserved unchanged when completion has not changed since the last observation.

### `estimatedCompletedAt`

An estimate of when the update will finish, computed only while an update is ongoing (assessment is not `Completed`).

**Baseline duration:** The duration of the most recent previously-completed update is used as the baseline. This is found by scanning `history[1 : len-1]` (skipping the current update at index 0 and the likely installation at the last index) for the first entry with `State == CompletedUpdate`. If no such entry exists (fewer than 3 history entries, or no completed prior update), the baseline defaults to **60 minutes**.

**Early phase** (less than 5 minutes of observed progress elapsed, or no progress detected yet): The estimate is computed as `baseline - elapsed`, with a 20% overestimate applied to the remaining time.

**Later phase** (more than 5 minutes of progress observed AND some ClusterOperators have updated): The estimate uses a polynomial curve that maps ClusterOperator completion percentage to a timewise completion percentage. The curve approximates observed real-world update progress. The remaining time is computed as `(elapsed / timewiseCompletion) - elapsed`, again with a 20% overestimate on positive remaining time (or 20% underestimate if already past the estimated completion time).

**Rounding:** Estimates greater than 10 minutes are rounded to the nearest minute; shorter estimates are rounded to the nearest second.

---

## 8. Update Optimization

Status is only written to the API server when there is a "significant difference" between the computed new status and the current status. This avoids unnecessary API traffic from small time-precision variations.

**Always significant** (triggers a write):
- Any change to `name`, `assessment`, or `completionPercent`
- Any change to version information (target version string, or presence/absence of previous version)
- Any change to a condition's `type`, `status`, `reason`, or `message`
- Transition of `completedAt` or `estimatedCompletedAt` between nil and non-nil

**Not significant** (write is skipped):
- Time field differences of less than **30 seconds** — applies to `startedAt`, `completedAt`, `estimatedCompletedAt`, `lastObservedProgress`, and condition `lastTransitionTime`

When no significant difference is found, the controller still proceeds to reconcile UpdateHealthInsights (see §9).

---

## 9. UpdateHealthInsight Lifecycle

UpdateHealthInsight CRDs capture discrete health observations. The controller manages them as owned resources of the ClusterVersionProgressInsight, using the label `insight-manager=clusterversion` to identify the set it owns.

Each reconciliation:
1. Lists all existing UpdateHealthInsights with the label `insight-manager=clusterversion`
2. Computes the desired set from the current ClusterVersion state
3. Creates insights that are desired but not yet present
4. Updates the status of insights that exist in both sets
5. Deletes insights that exist but are no longer desired

Insight names are derived from a hash of the insight's scope (resource references) and impact summary, prefixed with `cv-`.

**Currently implemented health insights:**

- **Forced health insight** (developer/testing feature): When a ClusterVersion resource carries the annotation `oue.openshift.muller.dev/force-health-insight`, the controller generates one health insight with `Info` impact level, `None` impact type, scoped to the ClusterVersion resource. This is intended for testing the health insight pipeline end-to-end and is not expected in production use.

No other health insight categories are currently implemented.

---

## Notes and Limitations

- The ClusterOperator filter only triggers on changes to the `"operator"` version field. Changes to other ClusterOperator fields (conditions, other version names, etc.) do not trigger reconciliation. There is a known gap: the filter does not distinguish between OCP payload ClusterOperators and non-payload ones; all ClusterOperators are counted toward completion.
- The `Degraded` assessment and the `Healthy` condition type are defined in the API but are not yet set by the controller.
- `EstimatedCompletedAt` relies on a polynomial approximation fitted from observed update data on one reference cluster; accuracy will vary across cluster sizes and configurations.
