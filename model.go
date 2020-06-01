package tsd

import "time"

//Document represents tsd document
type Document struct {
	ProcessID string    `json:"processid" db:"PROCESSID"`
	BaseDoc   string    `json:"basedoc" db:"BASEDOC"`
	PaperDoc  string    `json:"paperdoc" db:"PAPERDOC"`
	DBTime    time.Time `json:"dbtime" db:"DBTIME"`
	StartTime time.Time `json:"starttime" db:"STARTTIME"`
	EndTime   time.Time `json:"endtime" db:"ENDTIME"`
	UserID    int       `json:"userid" db:"USERID"`
	UserName  string    `json:"username" db:"USERNAME"`
	ResultDoc string    `json:"resultdoc" db:"RESULTDOC"`
	Vendor    string    `json:"vendor" db:"VENDOR"`
	Items     []DocItem `json:"-" db:"-"`
}

/*
SELECT d.processid,
       d.docor basedoc,
       d.supplierdoc paperdoc,
       d.starttime,
       d.endtime,
       d.terminaluser userid,
       (SELECT st.surname
          FROM smstaff st
         WHERE st.id = d.terminaluser)
          username,
       (SELECT pr.docid
          FROM smprocessdoccreateresult pr
         WHERE pr.processtype = d.processtype AND pr.processid = d.processid and rownum<2)
          resultdoc
  FROM supermag.smprocessheaderorcv d, supermag.smprocess p
 WHERE d.processtype = 'ORCV' AND d.processid = 96242 AND p.processtype = d.processtype AND p.processid = d.processid
*/

//DocItem represents tsd spec item
type DocItem struct {
	ProcessID string    `json:"processid" db:"PROCESSID"`
	ItemID    int       `json:"specitem" db:"SPECITEM"`
	Barcode   string    `json:"barcode" db:"BARCODE"`
	Article   string    `json:"article" db:"ARTICLE"`
	CardName  string    `json:"cardname" db:"CARDNAME"`
	Pack      float64   `json:"pack" db:"PACK"`
	QttPack   float64   `json:"qttpack" db:"QTTPACK"`
	Qtt       float64   `json:"qtt" db:"QTT"`
	EventTime time.Time `json:"eventtime" db:"EVENTTIME"`
}

/*
SELECT s.processid,
       s.specitem,
       s.barcode,
       s.article,
       c.name AS cardname,
       s.quantitybarcode pack,
       s.quantityscan qttpack,
       NVL (s.quantitybarcode, 1) * s.quantityscan qtt,
       s.datescan eventtime,
       s.timefitness timefitness
  FROM supermag.smprocesslogorcv s, supermag.svcardname c
 WHERE s.processtype = 'ORCV' AND s.processid = 96242 AND c.article = s.article
*/
