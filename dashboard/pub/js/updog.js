//window.setInterval(updateApplications, 5000);
window.setInterval(updateTimestamps, 1000);

var jsonStream = new EventSource('/api/streaming/applications?only_changes=true')
jsonStream.onmessage = processMessage

var appStream = null

var timestampUpdaters = {};

function processMessage(e) {
	var data = JSON.parse(e.data);
	//console.log(data);

	$.each(data.applications, function(an, app) {
		var appDiv = $('#app_'+an);
		if (appDiv.length == 0) {
			appDiv = $('#app_temp').clone().prop('id', 'app_'+an).removeClass('template');
			appDiv.children('.title').text(an);
			$('#content').append(appDiv);

			if (window.location.hash == "#"+an) {
				processHash();
			}

		}

		var appsums = appDiv.children('.app_sum')

		if (!app.degraded && !app.failed && !appsums.hasClass("up")) {
			appsums.addClass("up");
			appsums.removeClass('degraded').removeClass('failed');
		}

		if (app.degraded && !app.failed && !appsums.hasClass("degraded")) {
			appsums.addClass("degraded");
			appsums.removeClass('up').removeClass('failed');
		}

		if (app.failed && !appsums.hasClass("failed")) {
			appsums.addClass("failed");
			appsums.removeClass('up').removeClass('degraded');
		}

		var nupServText = app.services_up+"/"+app.services_total;
		appsums.first().children('span').last().filter(function() {
			return $(this).text() !== nupServText;
		}).text(nupServText);

		var nupInstText = app.instances_up+"/"+app.instances_total;
		appsums.last().children('span').last().filter(function() {
			return $(this).text() !== nupInstText;
		}).text(nupInstText);

		$.each(app.services, function(sn, serv) {
			$.each(serv.instances, function(i, inst) {
				var iid = jq(i);
				var downRec = $(iid);
				if (!inst.up && downRec.length == 0) {
					var dr = $('#downrecs tr.template').clone().addClass("downrec").prop('id', i).removeClass("template");
					var a = document.createElement('a');
					a.href = i;
					var host = a.host || i
					dr.children('td.app_name').text(an);
					dr.children('td.serv_name').text(sn);
					dr.children('td.host_name').text(host.split('.')[0]);
					dr.children('td.inst_name').text(i);
					dr.children('td.down').children('time').attr("title", inst.last_change);
					$('#downrecs').children('table').append(dr);
				}
				if (inst.up && downRec.length > 0) {
					downRec.remove();
				}
			});
		});

		updateTimestamps();

		if ($('.downrec').length > 0) {
			$('#downrecs').show();
		} else {
			$('#downrecs').hide();
		}

	});

	header = $('.header');
	if (!data.degraded && !data.failed && !header.hasClass('up')) {
		header.addClass('up').removeClass('failed').removeClass('degraded');
	} else if (!data.failed && data.degraded && !header.hasClass('degraded')) {
		header.addClass("degraded").removeClass('failed').removeClass('up');
	} else if (data.failed && !header.hasClass('failed')) {
		header.addClass('failed').removeClass('degraded').removeClass('up');
	}

	var aupText = data.applications_up+'/'+data.applications_total+' apps';
	$('.aup').filter(function() {
		return $(this).text() !== aupText
	}).text(aupText)
	var supText = data.services_up+'/'+data.services_total+' services';
	$('.sup').filter(function() {
		return $(this).text() !== supText
	}).text(supText)
	var iupText = data.instances_up+'/'+data.instances_total+' instances';
	$('.iup').filter(function() {
		return $(this).text() !== iupText
	}).text(iupText)
}

function processAppMessage(e) {
	var data = JSON.parse(e.data);

	$.each(data.services, function(sn, serv) {
		var servDiv = $('#serv_'+sn);
		if (servDiv.length == 0) {
			servDiv = $('#serv_temp').clone().prop('id', 'serv_'+sn).removeClass('template');
			servDiv.children('.title').text(sn);
			$('#service_list').append(servDiv);
		}
		$.each(serv.instances, function(ia, inst) {
			var iajq = jq('inst_'+ia);
			var instDiv = $(iajq)
			if (instDiv.length == 0) {
				instDiv = $('#inst_temp').clone().prop('id', 'inst_'+ia).removeClass('template');
				instDiv.children('.ind').text(ia);
				servDiv.children('.inst_table').children('table').append(instDiv);
			}

			instDiv.children('.rtd').text(toMsFormatted(inst.response_time));
			instDiv.children('.lcd').children('time').attr('title', inst.last_change);

			if (inst.up && !instDiv.hasClass("up")) {
				instDiv.addClass("up");
				instDiv.removeClass('failed');
			}

			if (!inst.up && !instDiv.hasClass("failed")) {
				instDiv.addClass("failed");
				instDiv.removeClass('up');
			}
		});

		var servTitle = servDiv.children('.title')

		if (!serv.degraded && !serv.failed && !servTitle.hasClass("up")) {
			servTitle.addClass("up");
			servTitle.removeClass('degraded').removeClass('failed');
		}

		if (serv.degraded && !serv.failed && !servTitle.hasClass("degraded")) {
			servTitle.addClass("degraded");
			servTitle.removeClass('up').removeClass('failed');
		}

		if (serv.failed && !servTitle.hasClass("failed")) {
			servTitle.addClass("failed");
			servTitle.removeClass('up').removeClass('degraded');
		}
	});

	updateTimestamps();
}

//document ready function: set up handlers
$(function() {
	$(document).keyup(function(e) {
		if (e.keyCode == 27) { //if esc key
			blankHash();
		}
	});

	$('#content').on('click', '.application', function() {
		window.location.hash = $(this).children('.title').text();
	});

	$('#modal').click(blankHash).children().click(function() { return false; });
	$('#modal_close').click(blankHash);

	$(window).bind('hashchange', processHash);
});

function processHash() {
	if (window.location.hash != "") {
		var hashapp = window.location.hash.split("#")[1];
		if (appStream == null) {
			//subscribe to application json stream
			appStream = new EventSource('/api/streaming/application/'+hashapp);
			appStream.onmessage = processAppMessage;
		} else {
			return;
		}

		$('#modal_title').text(hashapp);
		$('#modal').fadeIn('fast');
	} else {
		if (appStream != null) {
			appStream.close();
			appStream = null;
			$('#modal').fadeOut('fast', function() {
				$('#service_list').children('.service').not('.template').remove();
			});
		}
	}
}

function updateTimestamps() {
	$('time').filter(function() {
		var since = moment($(this).attr("title")).toNow(true)
		return $(this).text() !== since;
	}).each(function() {
		var since = moment($(this).attr("title")).toNow(true)
		$(this).text(since);
	});
}

function jq( myid ) {
	return "#" + myid.replace( /(\/|:|\.|\[|\]|,|=|@|\?)/g, "\\$1" );
}

function toMsFormatted(number) {
	return (Math.round(number/10000)/100).toFixed(2) + "ms";
}

function blankHash() { window.location.hash = ""; }
