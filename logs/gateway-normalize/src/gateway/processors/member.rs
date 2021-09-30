//! Defines processors for the following events:
//! - `MemberJoin`
//! - `MemberLeave`

use super::{extract, extract_id};
use crate::event::{Content, Entity, IdParams, Nickname, NormalizedEvent, Source, UserLike};
use crate::gateway::path::Path;
use crate::gateway::{Processor, ProcessorFleet};
use crate::rpc::logs::event::{EventOrigin, EventType};
use chrono::DateTime;
use lazy_static::lazy_static;
use std::convert::TryFrom;
use std::fmt;
use twilight_model::gateway::event::EventType as GatewayEventType;

lazy_static! {
    static ref ID_PATH: Path = Path::from("user.id");
    static ref JOINED_AT_PATH: Path = Path::from("joined_at");
    static ref USERNAME_PATH: Path = Path::from("user.username");
    static ref DISCRIMINATOR_PATH: Path = Path::from("user.discriminator");
    static ref NICKNAME_PATH: Path = Path::from("nick");
}

#[allow(clippy::too_many_lines)]
pub fn register_all(fleet: &mut ProcessorFleet) {
    // Register MemberJoin processor
    fleet.register(
        GatewayEventType::MemberAdd,
        Processor::sync(|source| {
            let ctx = source.context();

            let user_id = ctx.gateway(&ID_PATH, extract_id)?;
            let username = ctx.gateway(&USERNAME_PATH, extract::<String>).ok();
            let discriminator = ctx
                .gateway(&DISCRIMINATOR_PATH, extract::<String>)
                .ok()
                .and_then(|d| d.parse::<u16>().ok());
            let nickname = ctx
                .gateway(&NICKNAME_PATH, extract::<Option<String>>)
                .ok()
                .map(Nickname::from);
            let joined_at = ctx.gateway(&JOINED_AT_PATH, extract::<String>)?;
            let joined_at_date = DateTime::parse_from_rfc3339(&joined_at)?;
            let joined_at_ms_timestamp = u64::try_from(joined_at_date.timestamp_millis())?;

            let mut content = String::from("");
            write_mention(&mut content, user_id)?;
            content.push_str(" joined");

            Ok(NormalizedEvent {
                event_type: EventType::MemberJoin,
                id_params: IdParams::Two(user_id, joined_at_ms_timestamp),
                timestamp: joined_at_ms_timestamp,
                guild_id: ctx.event.guild_id,
                reason: None,
                audit_log_id: None,
                channel: None,
                agent: None,
                subject: Some(Entity::UserLike(UserLike {
                    id: user_id,
                    name: username,
                    nickname,
                    discriminator,
                    ..UserLike::default()
                })),
                auxiliary: None,
                content: Content {
                    inner: content,
                    users_mentioned: vec![user_id],
                    ..Content::default()
                },
                origin: EventOrigin::Gateway,
                // ctx to be dropped before we move inner out of source,
                // since ctx's inner field borrows source.inner
                source: Source {
                    gateway: Some(source.inner),
                    ..Source::default()
                },
            })
        }),
    );
    // Register MemberLeave processor
    fleet.register(
        GatewayEventType::MemberRemove,
        Processor::sync(|source| {
            let ctx = source.context();

            let user_id = ctx.gateway(&ID_PATH, extract_id)?;
            let username = ctx.gateway(&USERNAME_PATH, extract::<String>).ok();
            let discriminator = ctx
                .gateway(&DISCRIMINATOR_PATH, extract::<String>)
                .ok()
                .and_then(|d| d.parse::<u16>().ok());

            let mut content = String::from("");
            write_mention(&mut content, user_id)?;
            content.push_str(" left");

            Ok(NormalizedEvent {
                event_type: EventType::MemberLeave,
                id_params: IdParams::Two(user_id, ctx.event.ingress_timestamp),
                timestamp: ctx.event.ingress_timestamp,
                guild_id: ctx.event.guild_id,
                reason: None,
                audit_log_id: None,
                channel: None,
                agent: None,
                subject: Some(Entity::UserLike(UserLike {
                    id: user_id,
                    name: username,
                    discriminator,
                    ..UserLike::default()
                })),
                auxiliary: None,
                content: Content {
                    inner: content,
                    users_mentioned: vec![user_id],
                    ..Content::default()
                },
                origin: EventOrigin::Gateway,
                // ctx to be dropped before we move inner out of source,
                // since ctx's inner field borrows source.inner
                source: Source {
                    gateway: Some(source.inner),
                    ..Source::default()
                },
            })
        }),
    );
}

/// Writes a user mention that will be displayed using rich formatting
pub fn write_mention(writer: &mut impl fmt::Write, id: u64) -> Result<(), fmt::Error> {
    write!(writer, "<@{}>", id)
}
