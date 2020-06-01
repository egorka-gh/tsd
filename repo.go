package tsd

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

//Repository sm2000 database
type Repository interface {
	LoadDoc(ctx context.Context, docid string) (Document, error)
	LoadDocByTime(ctx context.Context, starttime time.Time, locid int) ([]Document, error)
	Close()
}

//NewRepo creates new Repository
func NewRepo(connection string) (Repository, error) {
	var db *sqlx.DB
	db, err := sqlx.Connect("godror", connection)
	if err != nil {
		return nil, err
	}

	return &basicRepository{
		db: db,
	}, nil

}

type basicRepository struct {
	db *sqlx.DB
}

func (b *basicRepository) Close() {
	b.db.Close()
}

func (b *basicRepository) LoadDoc(ctx context.Context, docid string) (Document, error) {
	var d Document
	/*
	   SELECT d.processid, d.docor basedoc, d.supplierdoc paperdoc, d.starttime, d.endtime, d.terminaluser userid,
	          (SELECT st.surname FROM smstaff st WHERE st.id = d.terminaluser) username,
	          (SELECT pr.docid FROM smprocessdoccreateresult pr WHERE pr.processtype = d.processtype AND pr.processid = d.processid and rownum<2) esultdoc
	     FROM supermag.smprocessheaderorcv d, supermag.smprocess p
	    WHERE d.processtype = 'ORCV' AND d.processid = 96242 AND p.processtype = d.processtype AND p.processid = d.processid
	*/
	var sb strings.Builder
	sb.WriteString("SELECT d.processid, d.docor basedoc, d.supplierdoc paperdoc, d.starttime, d.endtime, d.terminaluser userid, p.starttime dbtime,")
	sb.WriteString(" (SELECT st.surname FROM smstaff st WHERE st.id = d.terminaluser) username,")
	sb.WriteString(" (SELECT pr.docid FROM smprocessdoccreateresult pr WHERE pr.processtype = d.processtype AND pr.processid = d.processid and rownum<2) resultdoc,")
	sb.WriteString(" (SELECT NVL (cl.shortname, cl.name) FROM smdocuments dor, smclientinfo cl WHERE dor.doctype = 'OR' AND dor.id = d.docor AND cl.id = dor.clientindex) vendor")
	sb.WriteString(" FROM supermag.smprocessheaderorcv d, supermag.smprocess p")
	sb.WriteString(" WHERE d.processtype = 'ORCV' AND d.processid = :1")
	sb.WriteString(" AND p.processtype = d.processtype AND p.processid = d.processid")
	var ssql = sb.String()
	err := b.db.GetContext(ctx, &d, ssql, docid)
	if err != nil {
		return d, err
	}
	d.Items, err = b.LoadDocItems(ctx, docid)
	return d, err
}

func (b *basicRepository) LoadDocByTime(ctx context.Context, starttime time.Time, locid int) ([]Document, error) {
	var docs []Document
	var sb strings.Builder
	sb.WriteString("SELECT d.processid, d.docor basedoc, d.supplierdoc paperdoc, d.starttime, d.endtime, d.terminaluser userid, p.starttime dbtime,")
	sb.WriteString(" (SELECT st.surname FROM smstaff st WHERE st.id = d.terminaluser) username,")
	sb.WriteString(" (SELECT pr.docid FROM smprocessdoccreateresult pr WHERE pr.processtype = d.processtype AND pr.processid = d.processid and rownum<2) resultdoc,")
	sb.WriteString(" (SELECT NVL (cl.shortname, cl.name) FROM smdocuments dor, smclientinfo cl WHERE dor.doctype = 'OR' AND dor.id = d.docor AND cl.id = dor.clientindex) vendor")
	sb.WriteString(" FROM supermag.smprocessheaderorcv d, supermag.smprocess p")
	sb.WriteString(" WHERE d.processtype = 'ORCV' AND p.processtype = d.processtype AND p.processid = d.processid")
	sb.WriteString(" AND p.starttime > :1 AND :2 IN (d.location, -1)")
	var ssql = sb.String()
	err := b.db.SelectContext(ctx, &docs, ssql, starttime, locid)
	if err != nil {
		return docs, err
	}
	for i := range docs {
		docs[i].Items, err = b.LoadDocItems(ctx, docs[i].ProcessID)
		if err != nil {
			return []Document{}, err
		}
	}
	return docs, nil
}

func (b *basicRepository) LoadDocItems(ctx context.Context, docid string) ([]DocItem, error) {
	var res []DocItem
	/*
		SELECT s.processid, s.specitem, s.barcode, s.article, c.name cardname, s.quantitybarcode pack,
		       s.quantityscan qttpack, NVL(s.quantitybarcode, 1) * s.quantityscan qtt, s.datescan eventtime, s.timefitness timefitness
		  FROM supermag.smprocesslogorcv s, supermag.svcardname c
		 WHERE s.processtype = 'ORCV' AND s.processid = 96242 AND c.article = s.article
	*/
	var sb strings.Builder
	sb.WriteString("SELECT s.processid, s.specitem, s.barcode, s.article, c.name cardname, s.quantitybarcode pack,")
	sb.WriteString(" s.quantityscan qttpack, NVL(s.quantitybarcode, 1) * s.quantityscan qtt, s.datescan eventtime")
	sb.WriteString(" FROM supermag.smprocesslogorcv s, supermag.svcardname c")
	sb.WriteString(" WHERE s.processtype = 'ORCV' AND s.processid = :1")
	sb.WriteString(" AND c.article = s.article")
	sb.WriteString(" ORDER BY s.specitem")
	var ssql = sb.String()
	err := b.db.SelectContext(ctx, &res, ssql, docid)
	return res, err
}
