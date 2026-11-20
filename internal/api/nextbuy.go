package api

import (
	"sort"
	"strconv"
	"strings"

	"github.com/aarlint/ezlol/internal/ddragon"
)

// Next buy: the UI sends the build it is showing as an item path (core, boots,
// late options); ezlol finds the first item you do not own yet, credits the
// components you already carry towards it, and says what your gold buys now.

type itemRef struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	Gold  int    `json:"gold"` // cost to buy it now (full price, or what is left after owned parts)
}

type nextBuy struct {
	Target   itemRef   `json:"target"`
	Cost     int       `json:"cost"`     // full price of the target
	Have     int       `json:"have"`     // value of owned components that go into it
	Left     int       `json:"left"`     // Cost - Have: gold the finished item still needs
	Gold     int       `json:"gold"`     // your gold now
	Complete bool      `json:"complete"` // Gold >= Left: buy the finished item
	Buy      []itemRef `json:"buy"`      // components your gold covers now (when not Complete)
	BuyCost  int       `json:"buyCost"`
	After    []itemRef `json:"after"` // when Complete: what the gold left over buys towards the next item
	Then     []itemRef `json:"then"`  // the items after the target, in build order
	Done     int       `json:"done"`  // build items already owned
	Total    int       `json:"total"`
}

// parsePath reads "3031,3006,3046" (at most 12 ids).
func parsePath(q string) []int {
	var out []int
	for _, f := range strings.Split(q, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(f)); err == nil && id > 0 {
			out = append(out, id)
			if len(out) == 12 {
				break
			}
		}
	}
	return out
}

func components(d *ddragon.Data, id int) []int {
	var out []int
	for _, f := range d.Items[id].From {
		if c, err := strconv.Atoi(f); err == nil {
			if _, ok := d.Items[c]; ok {
				out = append(out, c)
			}
		}
	}
	return out
}

// buildsInto reports whether item `part` is anywhere in item `whole`'s recipe.
func buildsInto(d *ddragon.Data, part, whole int, depth int) bool {
	if depth > 6 {
		return false
	}
	for _, c := range components(d, whole) {
		if c == part || buildsInto(d, part, c, depth+1) {
			return true
		}
	}
	return false
}

// recipe is an item's tree with owned parts marked; cost is what is left to pay.
type recipe struct {
	id       int
	owned    bool
	cost     int
	children []*recipe
}

// tree expands id, consuming owned components from avail (so two recipes never
// count the same Long Sword twice). The root itself is never consumed.
func tree(d *ddragon.Data, id int, avail map[int]int, depth int) *recipe {
	r := &recipe{id: id, cost: d.Items[id].Gold}
	if depth > 6 {
		return r
	}
	for _, c := range components(d, id) {
		if avail[c] > 0 {
			avail[c]--
			r.children = append(r.children, &recipe{id: c, owned: true})
			r.cost -= d.Items[c].Gold
			continue
		}
		ch := tree(d, c, avail, depth+1)
		r.cost -= d.Items[c].Gold - ch.cost
		r.children = append(r.children, ch)
	}
	if r.cost < 0 {
		r.cost = 0
	}
	return r
}

// shop picks, top-down and biggest first, the unowned parts that gold covers.
func shop(r *recipe, gold int, picked *[]*recipe) int {
	kids := append([]*recipe(nil), r.children...)
	sort.SliceStable(kids, func(i, j int) bool { return kids[i].cost > kids[j].cost })
	for _, k := range kids {
		if k.owned || k.cost == 0 {
			continue
		}
		if k.cost <= gold {
			*picked = append(*picked, k)
			gold -= k.cost
			continue
		}
		gold = shop(k, gold, picked)
	}
	return gold
}

func planNextBuy(d *ddragon.Data, path []int, owned []int, gold int) *nextBuy {
	if d == nil || len(path) == 0 {
		return nil
	}
	have := map[int]int{}
	for _, id := range owned {
		have[id]++
	}
	ref := func(id, cost int) itemRef {
		it := d.Items[id]
		return itemRef{ID: id, Name: it.Name, Image: it.Image, Gold: cost}
	}
	// ownsOrBetter: the item itself, or something built from it (boots upgrades).
	ownsOrBetter := func(id int) bool {
		if have[id] > 0 {
			return true
		}
		for o := range have {
			if buildsInto(d, id, o, 0) {
				return true
			}
		}
		return false
	}
	nb := &nextBuy{Gold: gold}
	seen := map[int]bool{}
	target, second := 0, 0
	for _, id := range path {
		if _, ok := d.Items[id]; !ok || seen[id] {
			continue
		}
		seen[id] = true
		nb.Total++
		switch {
		case ownsOrBetter(id):
			nb.Done++
		case target == 0:
			target = id
		case len(nb.Then) < 3:
			if second == 0 {
				second = id
			}
			nb.Then = append(nb.Then, ref(id, d.Items[id].Gold))
		}
	}
	if target == 0 {
		return nb // the whole build is owned
	}
	avail := map[int]int{}
	for id, n := range have {
		avail[id] = n
	}
	r := tree(d, target, avail, 0)
	nb.Cost = d.Items[target].Gold
	nb.Left = r.cost
	nb.Have = nb.Cost - nb.Left
	nb.Target = ref(target, nb.Left)
	if gold >= nb.Left {
		nb.Complete = true
		// Spend the rest on the item after it (its own owned parts still count).
		if rest := gold - nb.Left; second != 0 && rest > 0 {
			r2 := tree(d, second, avail, 0)
			if rest >= r2.cost {
				nb.After = append(nb.After, ref(second, r2.cost))
			} else {
				var picked []*recipe
				shop(r2, rest, &picked)
				for _, p := range picked {
					nb.After = append(nb.After, ref(p.id, p.cost))
				}
			}
		}
		return nb
	}
	var picked []*recipe
	shop(r, gold, &picked)
	for _, p := range picked {
		nb.Buy = append(nb.Buy, ref(p.id, p.cost))
		nb.BuyCost += p.cost
	}
	return nb
}
