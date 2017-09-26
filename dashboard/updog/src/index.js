import React from 'react';
import ReactDOM from 'react-dom';
import axios from 'axios';
import moment from 'moment';
import './updog.css';

function Summary(props) {
	if (!props.total) { return(null) }
	return(
		<div className={["summary", props.mon_type].join(' ')}>
			{props.up}/{props.total} {props.mon_type}
		</div>
	)
}

function ResponseTime(props) {
	if (props.average_response_time) {
		return(<div className="response_time">{toMsFormatted(props.average_response_time) + "ms avg"}</div>)
	}
	if (props.response_time) {
		return(<div className="response_time">{toMsFormatted(props.response_time) + "ms"}</div>)
	}
	return(null)
}

function Timestamp(props) {
	if (!props.timestamp) { return(null) }
	const ts = moment(props.timestamp)
	const sago = (moment().unix() - ts.unix()) + "s ago"
	return(
		<div className="timestamp">{sago}
			<span className="tsTooltip">{props.timestamp}</span>
		</div>
	)
}

class Updog extends React.Component {
	constructor() {
		super();
		this.state = {
			mon_types: ["application", "service", "instance"],
			data: undefined,
		}
	}

	componentDidMount() {
		axios.get('http://updog.dc0.cl.trilliumstaffing.com:8080/api/status/applications')
			.then(res => {
				this.setState({ data: res.data })
			});
	}

	render() {
		const data = Object.assign({}, this.state.data);

		const summaries = this.state.mon_types.map((mon_type) => {
			return (
				<Summary
					key={mon_type + '_summary'}
					mon_type={mon_type + 's'}
					up={data[mon_type + 's_up']}
					total={data[mon_type + 's_total']}
				/>
			);
		});

		const content = parseData(data, this.state.mon_types)

		let _status = "up"
		if (data['up'] === false || data['failed'] === true) { _status = "failed" }
		if (data['degraded'] === true) { _status = "degraded" }

		return (
			<div>
			<div className={["header", _status].join(' ')}>
				<div className="title">What's UpDog?</div>
				{summaries}
			</div>
			<div className="content">{content}</div>
			</div>
		);
	}
}

ReactDOM.render(<Updog />, document.getElementById('root'));

function parseData(data, mon_types) {
	return mon_types.map((mon_type) => {
		if (! data[mon_type + 's']) { return(null) }
		const objs = Object.keys(data[mon_type + 's'] || {}).map((key) => {
			const t = data[mon_type + 's'][key]

			let _status = "up"
			if (t['up'] === false || t['failed'] === true) { _status = "failed" }
			if (t['degraded'] === true) { _status = "degraded" }

			const classes = [mon_type, _status].join(' ')

			const summaries = mon_types.map((mon_type) => {
				return (
					<Summary
						key={key+'_' + mon_type + '_summary'}
						mon_type={mon_type + 's'}
						up={t[mon_type + 's_up']}
						total={t[mon_type + 's_total']}
					/>
				);
			});

			const d = parseData(t, mon_types);

			return(
				<div className={mon_type}>
					<div className={[mon_type, "summary", _status].join(' ')}>
						<div className="title">{key}</div>
						<div className="status">{_status}</div>
						{summaries}
						<ResponseTime
							average_response_time={t.average_response_time}
							response_time={t.response_time}
						/>
						<Timestamp timestamp={t.timestamp} />
					</div>
					{d}
				</div>
			)
		});
		return <div className={mon_type + "s"}>{ objs }</div>;
	});
}

function toMsFormatted(number) {
	return (Math.round(number/10000)/100).toFixed(2);
}
