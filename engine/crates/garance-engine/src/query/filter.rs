use super::error::QueryError;

#[derive(Debug, Clone, PartialEq)]
pub enum Operator { Eq, Neq, Gt, Gte, Lt, Lte, Like, Ilike, Is, In }

impl Operator {
    pub fn from_str(s: &str) -> Result<Self, QueryError> {
        match s {
            "eq" => Ok(Operator::Eq), "neq" => Ok(Operator::Neq),
            "gt" => Ok(Operator::Gt), "gte" => Ok(Operator::Gte),
            "lt" => Ok(Operator::Lt), "lte" => Ok(Operator::Lte),
            "like" => Ok(Operator::Like), "ilike" => Ok(Operator::Ilike),
            "is" => Ok(Operator::Is), "in" => Ok(Operator::In),
            other => Err(QueryError::InvalidOperator(other.into())),
        }
    }
    pub fn to_sql(&self) -> &'static str {
        match self {
            Operator::Eq => "=", Operator::Neq => "!=",
            Operator::Gt => ">", Operator::Gte => ">=",
            Operator::Lt => "<", Operator::Lte => "<=",
            Operator::Like => "LIKE", Operator::Ilike => "ILIKE",
            Operator::Is => "IS", Operator::In => "IN",
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Filter { pub column: String, pub operator: Operator, pub value: String }

#[derive(Debug, Clone, PartialEq)]
pub enum SortDirection { Asc, Desc }

#[derive(Debug, Clone, PartialEq)]
pub struct Sort { pub column: String, pub direction: SortDirection }

#[derive(Debug, Clone, Default)]
pub struct QueryParams {
    pub select: Option<Vec<String>>,
    pub filters: Vec<Filter>,
    pub order: Vec<Sort>,
    pub limit: Option<i64>,
    pub offset: Option<i64>,
}

pub fn parse_query_params(params: &[(String, String)]) -> Result<QueryParams, QueryError> {
    let mut qp = QueryParams::default();
    for (key, value) in params {
        match key.as_str() {
            "select" => { qp.select = Some(value.split(',').map(|s| s.trim().to_string()).collect()); }
            "order" => {
                for part in value.split(',') {
                    let parts: Vec<&str> = part.trim().split('.').collect();
                    let column = parts[0].to_string();
                    let direction = match parts.get(1) { Some(&"desc") => SortDirection::Desc, _ => SortDirection::Asc };
                    qp.order.push(Sort { column, direction });
                }
            }
            "limit" => { qp.limit = Some(value.parse().map_err(|_| QueryError::InvalidValue { column: "limit".into(), reason: "must be an integer".into() })?); }
            "offset" => { qp.offset = Some(value.parse().map_err(|_| QueryError::InvalidValue { column: "offset".into(), reason: "must be an integer".into() })?); }
            column => {
                let dot_pos = value.find('.').ok_or_else(|| QueryError::InvalidValue { column: column.into(), reason: "filter must be in format operator.value (e.g., eq.Paris)".into() })?;
                let (op_str, val) = value.split_at(dot_pos);
                let val = &val[1..];
                let operator = Operator::from_str(op_str)?;
                qp.filters.push(Filter { column: column.to_string(), operator, value: val.to_string() });
            }
        }
    }
    Ok(qp)
}
