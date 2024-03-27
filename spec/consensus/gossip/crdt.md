# Tendermint Message sets as a CRDT

Here we argue that the exchange of messages of the Tendermint algorithm can and should be seen as an eventually convergent Tuple implemented as a tuple space CRDT.

> :warning:
> We assume that you understand the Tendermint algorithm and therefore we will not review it here.
If this is not the case, please refer to the [consensus.md](../consensus.md) document.

Three kinds of messages are exchanged in the Tendermint algorithm: `PROPOSAL`, `PRE-VOTE`, and `PRE-COMMIT`.
The algorithm progresses when certain conditions are satisfied over the set of messages received.
For example, in order do decide on a value `v`, the set must include a `PROPOSAL` for `v` and `PRE-COMMIT` for the same `v` from more than two thirds of the validators for the same round.
Since processes are subject to failures, correct processes cannot wait indefinitely for messages since the sender may be faulty.
Hence, processes execute in rounds in which they wait for conditions to be met for some time but, if they timeout, send negative messages that will lead to new rounds.

## On the need for the Gossip Communication property

Progress and termination are only guaranteed if there exists a **Global Stabilization Time (GST)** after which communication is reliable and timely (Eventual $\Delta$-Timely Communication).

| Eventual $\Delta$-Timely Communication|
|-----|
|There is a bound $\Delta$ and an instant GST (Global Stabilization Time) such that if a correct process $p$ sends a message $m$ at a time $t \geq \text{GST}$ to a correct process $q$, then $q$ will receive $m$ before $t + \Delta$.

The idea is that Eventual $\Delta$-Timely Communication be used to provide the **Gossip Communication property**, which ensures that all messages sent by correct processes will be eventually delivered to all correct processes.
This will, in turn, lead all correct processes to eventually be able to execute a round in which the conditions to decide are met, even if only after the GST is reached.

|Gossip Communication property|
|-----|
| (i) If a correct process $p$ sends some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max} (t,\text{GST}) + \Delta$.
| (ii) If a correct process $p$ receives some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max}(t,\text{GST}) + \Delta$.


Even if Eventual $\Delta$-Timely Communication is assumed, implementing the Gossip Communication property would be unfeasible.
Given that all messages, even messages sent before the GST, need to be buffered to be reliably delivered between correct processes and that GST may take indefinitely long to arrive, implementing this primitive would require unbounded memory.

Fortunately, while the Gossip Communication property is a sufficient condition for the Tendermint algorithm to terminate, it is not strictly necessary:
i) the conditions to progress and terminate are evaluated over the messages of subsets of rounds executed, not all of them; ii) as new rounds are executed, messages in previous rounds may be become obsolete and be ignored and forgotten.
In other words, the algorithm does not require all messages to be delivered, only messages that advance the state of the processes.

## Node's state as a Tuple Space

One way of looking at the information used by CometBFT nodes is as a distributed tuple space (a set of tuples) to which all nodes contribute.
Entries are added by validators over possibly many rounds of possibly many heights.
Each entry has form $\lang h, r, s, v, p \rang$ and corresponds to the message validator node $v$ sent in step $s$ of round $r$ of height $h$; $p$ is a tuple with the message payload.
In the algorithm, whenever a message would be broadcast now a tuple is added to the tuple space.

Because of the asynchronous nature of distributed systems, what a node's view of what is in the tuple space, its **local view**, will differ from the other nodes.
There are essentially two ways of making converging the local views of nodes.

- **Approach One**: nodes broadcast all the updates they want to perform to all nodes, including themselves.
If using Reliable Broadcast/the Gossip Communication property, the tuple space will eventually converge to include all broadcast messages.
- **Approach Two**: nodes periodically compare their local views with each other, 1-to-1, to identify and correct differences by adding missing entries, using some anti-entropy protocol.

These approaches work to reach convergence because the updates are commutative regarding the tuple space; each update simply adds an entry to a set.
From the Tendermint algorithm's point of view, convergence guarantees progress but is not a requirement for correctness.
In other words, nodes observing different local views of the tuple space may decide at different points in time but cannot violate any correctness guarantees and the eventual convergence of tuple space implies the eventual termination of the algorithm.


### Tuple Removal and Garbage Collection

In both approaches for synchronization, the tuple space could grow indefinitely, given that the number of heights and rounds is infinite.
To save memory, entries should be removed from the tuple space as soon as they become stale, that is, they are no longer useful.
For example, if a new height is started, all entries corresponding to previous heights become stale.

In general, simply forgetting stale entries in the local view would save the most space.
However, if the second approach described [above](#nodes-state-as-a-tuple-space), it could lead to entries being added back and never being completely purged from the system.
Although stale entries do not affect the algorithm, or they would not be considered stale, not adding the entries back is important for performance and resource utilization sake.

One way to prevent re-adding entries is keeping _tombstones_ for the removed entries.
A tombstone is nothing but an entry that supersedes a specific other entry.
Let $\bar{e}$ be the tombstone for an entry $e$; if, during synchronization, a node is informed of $e$ but it already has $\bar{e}$, then it does not add $e$ to its local view.

However small tombstones may be (for example, they could contain just the hash of the entry it supersedes), with time they will accrue and need to be garbage collected, in which case the corresponding entry may be added again; again, this will not break correctness and as long as tombstones are kept for long enough, the risk of re-adding becomes minimal.

In the case of the Tendermint algorithm we note that staleness comes from adding newer entries (belonging to higher rounds and heights) to the tuple space.
If, as an optimization to Approach Two, these newer entries are exchanged first, then the stale entries can be excluded before being shared to other nodes that might have forgotten them and tombstones may not be needed at all.

While gossiping of tombstones themselves could be useful, it adds the risk of malicious nodes using them to disrupt the system.
This could be prevented by tombstones carrying the set of entries that led to the entry removal, but these messages need to be gossiped as well, allowing the counterpart in the gossip to do its own cleanup and without needing the bloated tombstone at all; hence the tombstones should be local only.

### Equivocation and forgery

The originator validator of each tuple signs it before adding it to the tuple space therefore making it unfeasible to forge entries from other validators unless they collude, which does not add to the power of attacks.
Equivocation attacks are not averted, but will be detected once two tuples differing only on the payload are added to the same local view and may be used as evidence of misbehavior.[^todo1]
The Tendermint algorithm itself tolerates equivocation attacks within certain bounds.

[^todo1]: 1) Do we need anything more in terms of making this data structure byzantine fault tolerant? If looking this structure independently, then a byzantine node could simply add more and entries. From TM point of though, those messages won't cause any harm. 2) How does TM prevent a byz node from "flooding" the network with nill votes today? it does not, so we are not making the problem worse. 3)If needed, look into [Making CRDTs Byzantine Fault Tolerant](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf) for inspiration: "Many CRDTs, such as Logoot [44] and Treedoc [ 36], assign a unique identifier to each item (e.g. each element in a sequence); the data structure does not allow multiple items with the same ID, since then an ID would be ambiguous.” Can we use the payload itself as ID? However this work is focused on operation-based CRDT, not state-based.

### Querying the Tuple Space

The tuple space is consulted through queries, which have the same form as the entries.
Queries return all entries in their local views whose values match those in the query; `*` matches all values.
For example, suppose a node's local view of the tuple space has the following entries, here organized as rows of a table for easier visualization:

| | Height | Round | Step      | Validator | Payload        | |
|---| ------ | ----- | --------- | --------- | -------------- |---|
$\lang$ | 1      | 0     | Proposal  | v1        | pp1            |$\rang$|
$\lang$ | 1      | 0     | PreVote   | v1        | vp1            |$\rang$|
$\lang$ | 1      | 1     | PreCommit | v2        | cp1            |$\rang$|
$\lang$ | 2      | 0     | Proposal  | v1        | pp2            |$\rang$|
$\lang$ | 2      | 2     | PreVote   | v2        | vp2            |$\rang$|
$\lang$ | 2      | 3     | PreCommit | v2        | cp2   [^equiv] |$\rang$|
$\lang$ | 2      | 3     | PreCommit | v2        | cp2'  [^equiv] |$\rang$|

- Query $\lang 0, 0, Proposal, v1, * \rang$ returns $\{ \lang 0, 0, Proposal, v1, pp1 \rang \}$
- Query $\lang 0, 0, *, v1, * \rang$ returns $\{ \lang 0, 0, Proposal, v1, pp1 \rang,  \lang 0, 0, PreVote, v1, vp1 \rang \}$.

If needed for disambiguation, queries are tagged with the node being queried, as in $\lang 0, 0, *, v1, * \rang @p$

[^equiv]: These tuples are evidence of an equivocation attack.

#### State Validity

Let $V^h \subseteq P$ be the set of validators of height $h$ and $\pi^h_r \in V^h$ be the proposer of round $r$ of height $h$.
When the context eliminates any ambiguity on the height number, we might write these values simply as $V$ and $\pi_r$.

Given that each validator can execute each step only once per round, a query that specifies height, round, step and validator SHOULD either return empty or a single tuple.

- $\forall h \in \N, \forall r \in \N, \forall s \in \{\text{Proposal, PreVote, PreCommit}\}, \forall v \in V^h$,  $\cup_{p \in P} \lang h, r, s, v, * \rang_p$ contains at most one element.

In the specific case of the Proposal step, only the proposer of the round CAN have a matching entry.

- $\forall h \in \N, \forall r \in \N$, $\cup_{p \in P} \lang h, r, \text{Proposal}, *, * \rang_p$ contains at most one element and it also matches $\cup_{p\in P} \lang h, r, \text{Proposal}, \pi^h_r, * \rang_p$

A violation of these rules is a misbehavior by the validator signing the offending entries.

### Eventual Convergence

Consider the following definition for **Eventual Convergence**.

|Eventual Convergence|
|-----|
| If there exists a correct process $p \in P$ such that $e$ belongs to the local view of $p$, then, eventually, for every correct process $q \in P$, either $e$ belongs to or is stale in the local view of $q$.

In order to ensure convergence even in the presence of failures, the network must be connected in such a way to allow communication around any malicious nodes, that is, to provide paths connecting correct nodes.
Even if paths connecting correct nodes exist, effectively using them requires timeouts to not expire precociously and abort communication attempts.
Timeout values can be guaranteed to eventually be enough for communication after a GST is reached, which implies that all communication between correct processes will eventually happen timely, which implies that the tuple space will converge and keep converging.
Formally, if there is a GST then the following holds true:

| Eventual $\Delta$-Timely Convergence |
|---|
| If there exists a correct process $p \in P$ such that $e$ is the local view of $p$ at instant $t$, then by $\text{max}(t,\text{GST}) + \Delta$, for every correct process $q \in P$, either $e$ is in the local view of $q$ or is stale in the local view of $p$.|

Although GST may be too strong an expectation, in practice timely communication frequently happens within small stable periods, also leading to convergence.

### Why use a Tuple Space

Let's recall why we are considering using a tuple space to propagate Tendermint's messages.
It should be straightforward to see that Reliable Broadcast may be used to achieve Eventual Convergence and the Gossip Communication property may be used to implement Eventual $\Delta$-Timely Convergence:

- to add an entry to the tuple space, broadcast the entry;
- once delivered, add the entry to the local view.[^proof]

But if indeed we use the Gossip Communication property, then there are no obvious gains with respect to simply using broadcasts directly.

It should also be clear that if no entries are ever removed from the tuple space, then the inverse is also true:

- to broadcast a message, add it to the local view;
- once an entry is added to the local view, deliver it.

However, if entries can be removed, then the Tuple Space is actually weaker, since some entries may never be seen by some nodes, and should be easier/cheaper to implement.
We argue later that it can be implemented using Anti-Entropy or Epidemic protocols/Gossiping (not equal to the Gossip Communication property).
We pointed out [previously](#on-the-need-for-the-gossip-communication-property) that the Gossip Communication property is overkill for Tendermint because it requires even stale messages to be delivered.
We remove entries corresponding to stale messages and never deliver them.

[^proof]:  TODO: do we need to extend here?

### When to remove

> **TODO** Define conditions for tuple removal. Reference

## The tuple space as a CRDT

Conflict-free Replicated Data Types (CRDT) are distributed data structures that explore commutativity in update operations to achieve [Strong Eventual Consistency](https://en.wikipedia.org/wiki/Eventual_consistency#Strong_eventual_consistency).
As an example of CRDT, consider a counter updated by increment operations, known as Grown only counter (G-Counter): as long as the same set of operations are executed by two replicas, their views of the counter will be the same, irrespective of the execution order.

More relevant CRDT are the Grow only Set (G-Set), in which operations add elements to a set, and the [2-Phase Set](https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type#2P-Set_(Two-Phase_Set)) (2P-Set), which combines two G-Set to collect inclusions and exclusions to a set.

CRDT may be defined in two ways, operation- and state-based.
Operation-based CRDT use reliable communication to ensure that all updates (operations) are delivered to all replicas.
If the reliable communication primitive precludes duplication, then applying all operations will lead to the same state, irrespective of the delivery order since operations are commutative.
If duplications are allowed, then the operations must be made idempotent somehow.

State-based CRDT do not rely on reliable communication.
Instead they assume that replicas will compare their states and converge two-by-two using a merge function;
as long as the function is commutative, associative and idempotent, the states will converge.
For example, in the G-Set case, the merge operator is simply the union of the sets.

The two approaches for converging the message sets in the Tendermint algorithm described [earlier](#nodes-state-as-a-tuple-space), without the deletion of entries, correspond to the operation- and state-based [Grow-only Set](https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type#G-Set_(Grow-only_Set)) CRDT;
if removals must be handled, then using a 2P-Set is an option.
Next we introduce a CRDT in which some removals happen internally to the CRDT, based on the fact that the information in some entries becomes stale or has been superseded by the information in other entries present.

### About supersession

We call an entry of the tuple space an **Entry**[^tuple] and call **View** a set of entries.

[^tuple]: We use Entry instead of Tuple not to conflict with the use of tuple in Quint.

Let $v_1, v_2, v_3$ be views.
We say that $v_1$ is superseded by $v_2$ if all elements of $v_1$ are rendered stale by some subset of $v_2$ and note this relation as $v_1 < v_2$.
For all $e \in v_1$, if $v_1 < v_2$ then we abuse the notation and terminology and simply say that $e$ is superseded by $v_2$ and note it as $e < v_2$

The supersession $<$ relation must respect the following properties:[^not_contains]

[^not_contains]: $v_1 \subset v_2 \implies v_1 < v_2$ but $v_1 < v_2 \not\implies v_1 \subset v_2$

1. Transitivity: if $v_1 < v_2$ and $v_2 < v_3$, then $v_1 < v_3$;
1. Reflexiveness: $v < v$; [^reflex]
1. Anti-symmetry: if $v_1 < v_2$ and $v_3 < v_2$ then $v_1 = v_2$;
1. Tombstone validity: if $v_1 < v_2$, then $v_1$ is superseded by any sets obtained by replacing entries in $v_2$ by their corresponding tombstones;[^tomb_ss]

[^reflex]: TODO: do we really need this property?
[^tomb_ss]: In other words, $\{e\} < \{\bar{e}\}$

### Superseding Views CRDT

We define here the Superseding Views CRDT (SSE)[^name] as a set in which superseded elements are removed as new elements are added.
Since superseded elements are irrelevant from the point of view of CRDT users, they may be removed without prejudice as long as the CRDT properties are ensured.

[^name]: settle on a name.

We define SSE is as a state-based CRDT.
While an equivalent operations-based definition must exist, it is out of the scope of this document.


#### The merge operator - $\sqcup$

State based CRDT use a merge operation to combine two replica states (their views).
In our CRDT we embed the removal of superseded messages, as defined in the previous section, in the merge operation.

Let $\mathcal{V}$ be the power set of entries, or the set of all views of a system.
The merge operator $\sqcup: \mathcal{V} x \mathcal{V} \rightarrow \mathcal{V}$ is defined as the union of the input views, without the entries superseded by the other elements in the union, that is: $v_1 \sqcup v_2 = \{e: e \in (v_1 \cup v_2) \land e \not< (v_1 \cup v_2 \setminus \{e\})\}$

> :warning: TODO
>
> Prove that $\sqcup$ is  associative, commutative and idempotent;
>
> Show that $(\mathcal{V}, \cap, \cup_s)$ is a lattice
>
> - implies that there are no cycles;
> - For any $v_1 \in \mathcal{V}$ and entry $e$, $e < v_1$ implies that for any $v_2 \in \mathcal{V}$, $e < v_1 \sqcup v_2$ view.  
>  `view.forall(e => (e.isStale(view)).implies(e.isStale(removeStale(view.addEntry(e)))`
>
> Should be enough to QED.

#### SSE API

The SSE is generically defined in terms of

- `Entry`
    - a tuple/record of values;
    - application specific;
- `View`
    - a set of `Entry`;

 and the `isSupersededBy` function

- `isSupersededBy(v1: View, v2: View): bool`:
    - returns true iff all elements of `v1` are superseded by `v2`;
    - implements the $<$ relation;

- `matches(e:Entry, oe:OEntry):bool`
    - `OEntry` is a tuple/record such that, for each field of `Entry`, `OEntry` has a `Option` field with the same base type.
    - Returns true if each field of `e` equals the corresponding field of `oe` or the corresponding field of `oe` is `none`.

With these we can define the following operators:

- `removeStale(view: View): (View, View)`
    - returns a tuple `(v1,v2)` such that `v1` has the entries in `view` that are not superseded by the remainder elements and `v2` is `view` minus `v1`;
- `merge(lhs: View, rhs: View): View`
    - returns the result of `removeStale` applied to the union of `lhs` and `rhs`;
    - implements the $\sqcup$ relation;

The following helpers:

- `addEntry(v:View, e: Entry): View = merge(v, Set(e))`
    - returns the result of `removeStale` applied to the union of `view` and `{e}`;
- `query(v:view, oe:OEntry)`
    - Returns a view with the entries in `v` that match `oe`.
- `hasEntry(v: View, e:Entry):bool = removeStale(v).contains(e)`

And optionally:

- `delEntry(v:View, e: Entry): View`
    - `delEntry` adds tombstone for `e` to `view`;
    - `makeTs` must be provided
        - choose `et` such that
            - `isSuperseded({e}, {et})`
            - for any `e'` such that `isSuperseded({e'}, {et}) implies isSuperseded({e'}, {e})`
    - return `addEntry(v, makeTs(e))`;

#### Example 1

The `Lexy1` module in [sse.qnt](sse.qnt) specifies an SSE in which entries are tuples of three integer numbers.

An entry `(a,b,c)` is superseded by a view `v` if an only if `v` contains an entry  `(d,e,f)` such that

- either `a < d`
- or `a == d, b == e, c <= f` [^todoreflex]
- or `(d,e,f)` is superseded in `v`. [^todotrans]

[^todoreflex]: TODO, if reflex is dropped, this becomes `<`

[^todotrans]: TODO probably not needed, since whatever supersedes (d,e,f) will also supersede (a,b,c) directly.

The tuple could be interpreted as instance, node and round of some distributed protocol such that tuples from previous instances are obsoleted by tuples from new instances, and tuples from previous rounds are obsoleted by tuples for new rounds of the same node.

#### Example 2

The `Lexy2` module in [sse.qnt](sse.qnt) specifies an SSE in which entries are tuples of three integer numbers and a boolean.

In `Lexy2` an entry `(a,b,c,t)` is superseded by a view `v` if an only if `v` contains an entry  `(d,e,f,u)` such that

- either `a < d`
- or `a == d, b == e, c < f`
- or `a == d, b == e, c == f, t == u`,  [^todoreflex2]
- or `a == d, b == e, c == f, u == true`,
- or `(d,e,f,u)` is superseded in `v`. [^todotrans]

 [^todoreflex2]: TODO, if reflex is dropped, this is dropped.

As in previous example, the tuple could be interpreted as instance, node and round of some distributed protocol such that tuples from previous instances are obsoleted by tuples from new instances, and tuples from previous rounds are obsoleted by tuples for new rounds of the same node.
As an extra condition, the boolean acts as a tombstone marker for a tuple.

#### An SSE for Tendermint

> :warning: TODO
