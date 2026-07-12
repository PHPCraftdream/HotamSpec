// Package query is the compact agentic read interface over an
// ontology.Graph: card lookups (show), filtered rosters (list), ranked
// text search (search), one-hop neighborhoods (context), and bare
// neighbor lists (related). It never mutates the graph — every function
// here is a pure read projection, meant to replace an agent reading the
// full graph.json or a generated Markdown doc just to answer "what is
// R-x" or "what touches R-x".
package query
