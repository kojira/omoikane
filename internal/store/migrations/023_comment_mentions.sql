-- @mentions on entry comments — the addressing for review requests. See
-- design.md §23.21.
--
-- A comment can name who it is FOR (by user id or by librarian role, e.g.
-- "detective"). Only mentioned users get a review request — commenting
-- without a mention does NOT ping the entry's author, so a busy agent
-- isn't interrupted by every passing remark. The mention array is the
-- whole notification contract: X-Review-Requests counts unresolved
-- comments that mention you (and that you didn't write).
--
-- Stored as a JSON array of strings on the comment (queried with
-- json_each); the volume is small enough that a normalised join table
-- isn't warranted yet.

ALTER TABLE entry_comments ADD COLUMN mentions TEXT;
